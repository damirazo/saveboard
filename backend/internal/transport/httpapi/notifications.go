package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"github.com/danielgtaylor/huma/v2"
	"net/http"
	"saveboard/internal/notifications"
)

type NotificationsInput struct {
	Authorization string `header:"Authorization"`
	Page          int    `query:"page" minimum:"1" maximum:"10000" default:"1"`
	Size          int    `query:"size" minimum:"1" maximum:"50" default:"10"`
}
type NotificationsOutput struct {
	Body notifications.NotificationPage
}
type NotificationOutput struct{ Body notifications.Notification }
type NotificationCountOutput struct {
	Body struct {
		Count int `json:"count"`
	}
}

func notificationError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return huma.Error404NotFound("Notification not found")
	}
	return huma.Error503ServiceUnavailable("Notifications unavailable")
}
func registerNotifications(api huma.API, deps Dependencies) {
	viewer := func(ctx context.Context, header string) (string, error) {
		if deps.Auth == nil {
			return "", huma.Error401Unauthorized("Sign in required")
		}
		p, err := deps.Auth.Authenticate(ctx, bearer(header))
		if err != nil {
			return "", authError(err)
		}
		if deps.Notifications == nil {
			return "", huma.Error503ServiceUnavailable("Notifications unavailable")
		}
		return p.UserID.String(), nil
	}
	op := func(id, method, path string) huma.Operation {
		return huma.Operation{OperationID: id, Method: method, Path: path, Tags: []string{"Notifications"}, Security: []map[string][]string{{"bearerAuth": {}}}, Errors: []int{401, 403, 404, 422, 503}}
	}
	huma.Register(api, op("notifications-list", http.MethodGet, "/api/v1/notifications"), func(ctx context.Context, in *NotificationsInput) (*NotificationsOutput, error) {
		user, e := viewer(ctx, in.Authorization)
		if e != nil {
			return nil, e
		}
		v, e := deps.Notifications.List(ctx, user, in.Page, in.Size)
		if e != nil {
			return nil, notificationError(e)
		}
		return &NotificationsOutput{v}, nil
	})
	huma.Register(api, op("notifications-unread", http.MethodGet, "/api/v1/notifications/unread"), func(ctx context.Context, in *struct {
		Authorization string `header:"Authorization"`
	}) (*NotificationCountOutput, error) {
		user, e := viewer(ctx, in.Authorization)
		if e != nil {
			return nil, e
		}
		count, e := deps.Notifications.Unread(ctx, user)
		if e != nil {
			return nil, notificationError(e)
		}
		out := &NotificationCountOutput{}
		out.Body.Count = count
		return out, nil
	})
	huma.Register(api, op("notifications-get", http.MethodGet, "/api/v1/notifications/{id}"), func(ctx context.Context, in *PostIDInput) (*NotificationOutput, error) {
		user, e := viewer(ctx, in.Authorization)
		if e != nil {
			return nil, e
		}
		v, e := deps.Notifications.Get(ctx, user, in.ID)
		if e != nil {
			return nil, notificationError(e)
		}
		return &NotificationOutput{v}, nil
	})
	huma.Register(api, op("notifications-read", http.MethodPost, "/api/v1/notifications/{id}/read"), func(ctx context.Context, in *PostIDInput) (*struct{}, error) {
		user, e := viewer(ctx, in.Authorization)
		if e != nil {
			return nil, e
		}
		if e = deps.Notifications.Read(ctx, user, in.ID); e != nil {
			return nil, notificationError(e)
		}
		return &struct{}{}, nil
	})
}
