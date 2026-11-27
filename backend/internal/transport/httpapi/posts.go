package httpapi

import (
	"context"
	"errors"
	"net/http"
	"saveboard/internal/jobs"
	"saveboard/internal/posts"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type PostInput struct {
	Authorization string `header:"Authorization"`
	ID            string `path:"id" format:"uuid"`
	Body          posts.Write
}
type PostCreateInput struct {
	Authorization string `header:"Authorization"`
	Body          posts.Write
}
type PostIDInput struct {
	Authorization string `header:"Authorization"`
	ID            string `path:"id" format:"uuid"`
}
type PostOutput struct{ Body posts.View }
type CopyOutput struct{ Body jobs.CaptureResult }
type PostsInput struct {
	Author        string `query:"author" maxLength:"64" doc:"Filter by author username"`
	Authorization string `header:"Authorization"`
	Scope         string `query:"scope" enum:"public,mine" default:"public"`
	Type          string `query:"type" enum:"note,link,photo"`
	Query         string `query:"q" maxLength:"200"`
	IncludedTags  string `query:"include_tags" maxLength:"739" doc:"Comma-separated existing tag UUIDs; all required, at most 20"`
	ExcludedTags  string `query:"exclude_tags" maxLength:"739" doc:"Comma-separated tag UUIDs; any match excludes a post, at most 20"`
	Sort          string `query:"sort" enum:"new,old" default:"new"`
	Page          int    `query:"page" minimum:"1" maximum:"10000" default:"1"`
	Size          int    `query:"size" minimum:"1" maximum:"50" default:"10"`
}
type PostsOutput struct{ Body posts.Page }
type TagsOutput struct{ Body []posts.Tag }
type UploadInput struct {
	Authorization string `header:"Authorization"`
	Body          struct {
		Data []byte `json:"data" doc:"Base64 image, at most 10 MiB decoded"`
	}
}
type UploadOutput struct{ Body posts.PhotoView }
type PhotoOutput struct {
	ContentType string `header:"Content-Type"`
	Body        []byte
}

func postError(err error) error {
	switch {
	case errors.Is(err, posts.ErrInvalid):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, posts.ErrForbidden):
		return huma.Error403Forbidden("Only an active owner may change this post")
	case errors.Is(err, posts.ErrNotFound):
		return huma.Error404NotFound("Not found")
	default:
		return huma.Error503ServiceUnavailable("Posts temporarily unavailable")
	}
}
func registerPosts(api huma.API, deps Dependencies) {
	viewer := func(ctx context.Context, header string, required bool) (string, error) {
		if deps.Posts == nil {
			return "", huma.Error503ServiceUnavailable("Posts unavailable")
		}
		if header == "" && !required {
			return "", nil
		}
		if deps.Auth == nil {
			return "", huma.Error401Unauthorized("Sign in required")
		}
		p, err := deps.Auth.Authenticate(ctx, bearer(header))
		if err != nil {
			return "", authError(err)
		}
		return p.UserID.String(), nil
	}
	op := func(id, method, path string, protected bool) huma.Operation {
		o := huma.Operation{OperationID: id, Method: method, Path: path, Tags: []string{"Posts"}, Errors: []int{401, 403, 404, 413, 422, 503}}
		if protected {
			o.Security = []map[string][]string{{"bearerAuth": {}}}
		}
		return o
	}
	huma.Register(api, op("posts-list", http.MethodGet, "/api/v1/posts", false), func(ctx context.Context, in *PostsInput) (*PostsOutput, error) {
		who, e := viewer(ctx, in.Authorization, in.Scope == "mine")
		if e != nil {
			return nil, e
		}
		v, e := deps.Posts.List(ctx, who, in.Scope, in.Type, in.Query, in.Sort, in.Page, in.Size, in.IncludedTags, in.ExcludedTags, in.Author)
		if e != nil {
			return nil, postError(e)
		}
		return &PostsOutput{v}, nil
	})
	huma.Register(api, op("posts-create", http.MethodPost, "/api/v1/posts", true), func(ctx context.Context, in *PostCreateInput) (*PostOutput, error) {
		who, e := viewer(ctx, in.Authorization, true)
		if e != nil {
			return nil, e
		}
		v, e := deps.Posts.Save(ctx, who, "", in.Body)
		if e != nil {
			return nil, postError(e)
		}
		return &PostOutput{v}, nil
	})
	huma.Register(api, op("posts-update", http.MethodPut, "/api/v1/posts/{id}", true), func(ctx context.Context, in *PostInput) (*PostOutput, error) {
		who, e := viewer(ctx, in.Authorization, true)
		if e != nil {
			return nil, e
		}
		v, e := deps.Posts.Save(ctx, who, in.ID, in.Body)
		if e != nil {
			return nil, postError(e)
		}
		return &PostOutput{v}, nil
	})
	huma.Register(api, op("posts-get", http.MethodGet, "/api/v1/posts/{id}", false), func(ctx context.Context, in *PostIDInput) (*PostOutput, error) {
		who, e := viewer(ctx, in.Authorization, false)
		if e != nil {
			return nil, e
		}
		v, e := deps.Posts.Get(ctx, who, in.ID)
		if e != nil {
			return nil, postError(e)
		}
		return &PostOutput{v}, nil
	})
	huma.Register(api, op("posts-draft", http.MethodDelete, "/api/v1/posts/{id}", true), func(ctx context.Context, in *PostIDInput) (*struct{}, error) {
		who, e := viewer(ctx, in.Authorization, true)
		if e != nil {
			return nil, e
		}
		if e = deps.Posts.Draft(ctx, who, in.ID); e != nil {
			return nil, postError(e)
		}
		return nil, nil
	})
	huma.Register(api, op("tags-suggest", http.MethodGet, "/api/v1/post-tags", false), func(ctx context.Context, in *struct {
		Authorization string `header:"Authorization"`
		Query         string `query:"q" maxLength:"80"`
	}) (*TagsOutput, error) {
		if _, e := viewer(ctx, in.Authorization, false); e != nil {
			return nil, e
		}
		tags, e := deps.Posts.Tags(ctx, in.Query)
		if e != nil {
			return nil, postError(e)
		}
		return &TagsOutput{tags}, nil
	})
	huma.Register(api, op("posts-copy", http.MethodGet, "/api/v1/posts/{id}/copy", false), func(ctx context.Context, in *PostIDInput) (*CopyOutput, error) {
		who, e := viewer(ctx, in.Authorization, false)
		if e != nil {
			return nil, e
		}
		v, e := deps.Posts.Copy(ctx, who, in.ID)
		if e != nil {
			return nil, postError(e)
		}
		return &CopyOutput{v}, nil
	})
	huma.Register(api, op("posts-thumbnail", http.MethodGet, "/api/v1/posts/{id}/thumbnail", false), func(ctx context.Context, in *PostIDInput) (*PhotoOutput, error) {
		who, e := viewer(ctx, in.Authorization, false)
		if e != nil {
			return nil, e
		}
		v, e := deps.Posts.Thumbnail(ctx, who, in.ID)
		if e != nil {
			return nil, postError(e)
		}
		return &PhotoOutput{"image/png", v}, nil
	})
	uploadOp := op("photos-upload", http.MethodPost, "/api/v1/photos", true)
	uploadOp.MaxBodyBytes = 14 << 20
	// Authenticate before Huma reads/decodes the large JSON body.
	uploadOp.Middlewares = huma.Middlewares{func(ctx huma.Context, next func(huma.Context)) {
		if _, e := viewer(ctx.Context(), ctx.Header("Authorization"), true); e != nil {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "Sign in required")
			return
		}
		next(ctx)
	}}
	huma.Register(api, uploadOp, func(ctx context.Context, in *UploadInput) (*UploadOutput, error) {
		who, e := viewer(ctx, in.Authorization, true)
		if e != nil {
			return nil, e
		}
		p, e := deps.Posts.Upload(ctx, who, in.Body.Data)
		if e != nil {
			return nil, postError(e)
		}
		return &UploadOutput{p}, nil
	})
	huma.Register(api, op("photos-get", http.MethodGet, "/api/v1/photos/{id}", false), func(ctx context.Context, in *PostIDInput) (*PhotoOutput, error) {
		if _, e := uuid.Parse(in.ID); e != nil {
			return nil, huma.Error404NotFound("Not found")
		}
		who, e := viewer(ctx, in.Authorization, false)
		if e != nil {
			return nil, e
		}
		data, mime, e := deps.Posts.Photo(ctx, who, in.ID)
		if e != nil {
			return nil, postError(e)
		}
		return &PhotoOutput{mime, data}, nil
	})
}
