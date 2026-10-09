package httpapi

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"net/http"
	"saveboard/internal/identity"
	"saveboard/internal/posts"
	"time"
)

type Dependencies struct {
	Ping          func(context.Context) error
	Auth          *identity.AuthService
	Posts         *posts.Service
	SecureCookies bool
}
type HealthOutput struct {
	Body struct {
		Status string `json:"status"`
	}
}
type InfoOutput struct {
	Body struct {
		Name           string `json:"name"`
		Stage          string `json:"stage"`
		Authentication bool   `json:"authentication"`
		Capture        bool   `json:"capture"`
		SemanticSearch bool   `json:"semanticSearch"`
	}
}

// New is also used by cmd/openapi, so exporting the contract needs no services.
func New(deps Dependencies) (http.Handler, huma.API) {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Referrer-Policy", "no-referrer")
			limit := int64(1 << 20)
			if req.Method == http.MethodPost && req.URL.Path == "/api/v1/photos" {
				limit = 14 << 20
			}
			req.Body = http.MaxBytesReader(w, req.Body, limit)
			next.ServeHTTP(w, req)
		})
	})
	c := huma.DefaultConfig("Saveboard API", "0.1.0")
	c.OpenAPIPath = "/api/openapi"
	c.DocsPath = "/api/docs"
	c.SchemasPath = "/api/schemas"
	a := humachi.New(r, c)
	registerAuth(a, deps)
	registerPosts(a, deps)
	huma.Get(a, "/api/health/live", func(ctx context.Context, _ *struct{}) (*HealthOutput, error) {
		out := &HealthOutput{}
		out.Body.Status = "ok"
		return out, nil
	})
	huma.Register(a, huma.Operation{OperationID: "readiness", Method: http.MethodGet, Path: "/api/health/ready", Errors: []int{503}}, func(ctx context.Context, _ *struct{}) (*HealthOutput, error) {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if deps.Ping == nil || deps.Ping(ctx) != nil {
			return nil, huma.Error503ServiceUnavailable("database unavailable")
		}
		out := &HealthOutput{}
		out.Body.Status = "ready"
		return out, nil
	})
	huma.Get(a, "/api/v1/system", func(ctx context.Context, _ *struct{}) (*InfoOutput, error) {
		out := &InfoOutput{}
		out.Body.Name = "Saveboard"
		out.Body.Stage = "foundation"
		out.Body.Authentication = deps.Auth != nil
		return out, nil
	})
	return otelhttp.NewHandler(r, "http"), a
}
