package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"saveboard/internal/identity"
)

type LoginInput struct {
	Body struct {
		Login    string `json:"login" minLength:"1" maxLength:"254" doc:"Username or email (case-insensitive)"`
		Password string `json:"password" minLength:"1" maxLength:"72" format:"password"`
	}
}

type RegistrationInput struct {
	Body struct {
		Username string `json:"username" minLength:"3" maxLength:"64"`
		Email    string `json:"email" minLength:"3" maxLength:"254"`
		Password string `json:"password" minLength:"8" maxLength:"72" format:"password"`
	}
}
type RefreshInput struct {
	Body struct {
		RefreshToken string `json:"refresh_token" minLength:"43" maxLength:"43"`
	}
}
type TokenOutput struct{ Body identity.Tokens }
type AuthorizedInput struct {
	Authorization string `header:"Authorization"`
}
type PrincipalOutput struct{ Body identity.Principal }
type RevokeOutput struct {
	Body struct {
		Revoked int64 `json:"revoked"`
	}
}

func bearer(value string) string {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func authError(err error) error {
	switch {
	case errors.Is(err, identity.ErrRegistrationInvalid):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, identity.ErrAccountExists):
		return huma.Error409Conflict("Username or email is already registered")
	case errors.Is(err, identity.ErrUnauthorized):
		return huma.Error401Unauthorized("Invalid credentials or session")
	case errors.Is(err, identity.ErrBusy):
		return huma.Error429TooManyRequests("Too many login attempts")
	case errors.Is(err, identity.ErrOAuthUnavailable):
		return huma.Error503ServiceUnavailable("OAuth is not configured")
	default:
		// Do not log upstream OAuth response bodies, credentials, or tokens.
		slog.Error("authentication operation failed")
		return huma.Error503ServiceUnavailable("Authentication temporarily unavailable")
	}
}

func registerAuth(api huma.API, deps Dependencies) {
	api.OpenAPI().Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearerAuth": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
	}
	available := func() error {
		if deps.Auth == nil {
			return huma.Error503ServiceUnavailable("Authentication is not configured")
		}
		return nil
	}
	op := func(id, method, path string, protected bool) huma.Operation {
		o := huma.Operation{OperationID: id, Method: method, Path: path, Tags: []string{"Authentication"}, Errors: []int{401, 409, 422, 429, 503}}
		if protected {
			o.Security = []map[string][]string{{"bearerAuth": {}}}
		}
		return o
	}
	profileOp := op("public-profile", http.MethodGet, "/api/v1/profiles/{username}", false)
	profileOp.Errors = []int{404, 422, 503}
	huma.Register(api, profileOp, func(ctx context.Context, in *struct {
		Username string `path:"username" maxLength:"64"`
	}) (*struct{ Body identity.PublicProfile }, error) {
		if err := available(); err != nil {
			return nil, err
		}
		profile, err := deps.Auth.PublicProfile(ctx, in.Username)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("Profile not found")
		}
		if err != nil {
			return nil, authError(err)
		}
		return &struct{ Body identity.PublicProfile }{profile}, nil
	})
	huma.Register(api, op("profile-update", http.MethodPatch, "/api/v1/auth/me", true), func(ctx context.Context, in *struct {
		Authorization string `header:"Authorization"`
		Body          struct {
			DisplayName string `json:"display_name" minLength:"1" maxLength:"100"`
		}
	}) (*struct{}, error) {
		if err := available(); err != nil {
			return nil, err
		}
		p, err := deps.Auth.Authenticate(ctx, bearer(in.Authorization))
		if err != nil {
			return nil, authError(err)
		}
		if err = deps.Auth.UpdateDisplayName(ctx, p.UserID, in.Body.DisplayName); err != nil {
			return nil, authError(err)
		}
		return nil, nil
	})
	registrationOp := op("auth-register", http.MethodPost, "/api/v1/auth/register", false)
	registrationOp.DefaultStatus = http.StatusCreated
	huma.Register(api, registrationOp, func(ctx context.Context, in *RegistrationInput) (*TokenOutput, error) {
		if err := available(); err != nil {
			return nil, err
		}
		tokens, err := deps.Auth.Register(ctx, in.Body.Username, in.Body.Email, in.Body.Password)
		if err != nil {
			return nil, authError(err)
		}
		return &TokenOutput{Body: tokens}, nil
	})
	huma.Register(api, op("auth-login", http.MethodPost, "/api/v1/auth/login", false), func(ctx context.Context, in *LoginInput) (*TokenOutput, error) {
		if err := available(); err != nil {
			return nil, err
		}
		tokens, err := deps.Auth.Login(ctx, in.Body.Login, in.Body.Password)
		if err != nil {
			return nil, authError(err)
		}
		return &TokenOutput{Body: tokens}, nil
	})
	huma.Register(api, op("auth-refresh", http.MethodPost, "/api/v1/auth/refresh", false), func(ctx context.Context, in *RefreshInput) (*TokenOutput, error) {
		if err := available(); err != nil {
			return nil, err
		}
		tokens, err := deps.Auth.Refresh(ctx, in.Body.RefreshToken)
		if err != nil {
			return nil, authError(err)
		}
		return &TokenOutput{Body: tokens}, nil
	})
	huma.Register(api, op("auth-me", http.MethodGet, "/api/v1/auth/me", true), func(ctx context.Context, in *AuthorizedInput) (*PrincipalOutput, error) {
		if err := available(); err != nil {
			return nil, err
		}
		p, err := deps.Auth.Authenticate(ctx, bearer(in.Authorization))
		if err != nil {
			return nil, authError(err)
		}
		return &PrincipalOutput{Body: p}, nil
	})
	for _, route := range []struct {
		path, id string
		others   bool
	}{
		{"/api/v1/auth/logout", "auth-logout", false},
		{"/api/v1/auth/logout-others", "auth-logout-others", true},
	} {
		huma.Register(api, op(route.id, http.MethodPost, route.path, true), func(ctx context.Context, in *AuthorizedInput) (*RevokeOutput, error) {
			if err := available(); err != nil {
				return nil, err
			}
			count, err := deps.Auth.Revoke(ctx, bearer(in.Authorization), route.others)
			if err != nil {
				return nil, authError(err)
			}
			out := &RevokeOutput{}
			out.Body.Revoked = count
			return out, nil
		})
	}

	// A browser cookie is used only to bind the OAuth callback, never to authorize API mutations.
	cookie := func(value string, maxAge int) string {
		return (&http.Cookie{Name: "saveboard_oauth", Value: value, Path: "/auth/callback", HttpOnly: true,
			Secure: deps.SecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: maxAge}).String()
	}
	startOp := op("auth-oauth-start", http.MethodGet, "/auth/login", false)
	startOp.DefaultStatus = http.StatusFound
	huma.Register(api, startOp, func(ctx context.Context, _ *struct{}) (*OAuthRedirectOutput, error) {
		if err := available(); err != nil {
			return nil, err
		}
		start, err := deps.Auth.StartOAuth(ctx)
		if err != nil {
			return nil, authError(err)
		}
		return &OAuthRedirectOutput{Location: start.URL, SetCookie: cookie(start.BrowserToken, int(identity.FlowLifetime.Seconds()))}, nil
	})
	huma.Register(api, op("auth-oauth-callback", http.MethodGet, "/auth/callback", false), func(ctx context.Context, in *OAuthCallbackInput) (*OAuthTokenOutput, error) {
		if err := available(); err != nil {
			return nil, err
		}
		if in.Error != "" {
			return nil, huma.Error401Unauthorized("OAuth authorization failed")
		}
		req := &http.Request{Header: http.Header{"Cookie": {in.Cookie}}}
		browser, err := req.Cookie("saveboard_oauth")
		if err != nil {
			return nil, huma.Error401Unauthorized("Invalid OAuth browser session")
		}
		tokens, err := deps.Auth.FinishOAuth(ctx, in.State, browser.Value, in.Code)
		if err != nil {
			return nil, authError(err)
		}
		return &OAuthTokenOutput{Body: tokens, SetCookie: cookie("", -1)}, nil
	})
}

type OAuthRedirectOutput struct {
	Location  string `header:"Location"`
	SetCookie string `header:"Set-Cookie"`
}
type OAuthCallbackInput struct {
	State  string `query:"state" maxLength:"43"`
	Code   string `query:"code" maxLength:"8192"`
	Error  string `query:"error" maxLength:"256"`
	Cookie string `header:"Cookie"`
}
type OAuthTokenOutput struct {
	SetCookie string `header:"Set-Cookie"`
	Body      identity.Tokens
}
