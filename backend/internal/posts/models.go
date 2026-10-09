package posts

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusArchived  Status = "archived"
)

type Type string

const (
	TypeNote  Type = "note"
	TypeLink  Type = "link"
	TypePhoto Type = "photo"
)

type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityUnlisted Visibility = "unlisted"
	VisibilityFriends  Visibility = "friends"
	VisibilityPrivate  Visibility = "private"
)

type Post struct {
	bun.BaseModel `bun:"table:posts"`
	ID            uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AuthorID      uuid.UUID  `bun:"author_id,type:uuid,notnull" json:"author_id"`
	CreatedAt     time.Time  `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at,notnull,default:current_timestamp" json:"updated_at"`
	Status        Status     `bun:"status,notnull,default:'draft'" json:"status"`
	Type          Type       `bun:"type,notnull" json:"type"`
	Visibility    Visibility `bun:"visibility,notnull,default:'private'" json:"visibility"`
	Note          *Note      `bun:"rel:has-one,join:id=post_id" json:"note,omitempty"`
	Link          *Link      `bun:"rel:has-one,join:id=post_id" json:"link,omitempty"`
	Photos        []Photo    `bun:"rel:has-many,join:id=post_id" json:"photos,omitempty"`
	TagLinks      []PostTag  `bun:"rel:has-many,join:id=post_id" json:"tag_links,omitempty"`
}

// Subtype rows share their post's UUID. Type is checked by a composite foreign key.
type Note struct {
	bun.BaseModel `bun:"table:post_notes"`
	PostID        uuid.UUID `bun:"post_id,pk,type:uuid" json:"post_id"`
	Type          Type      `bun:"type,notnull,default:'note'" json:"-"`
	Text          string    `bun:"text,notnull,default:''" json:"text"`
}

type Link struct {
	bun.BaseModel `bun:"table:post_links"`
	PostID        uuid.UUID `bun:"post_id,pk,type:uuid" json:"post_id"`
	Type          Type      `bun:"type,notnull,default:'link'" json:"-"`
	URL           string    `bun:"url,notnull" json:"url"`
}

type Photo struct {
	bun.BaseModel `bun:"table:post_photos"`
	ID            uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	PostID        uuid.UUID `bun:"post_id,type:uuid,notnull" json:"post_id"`
	Type          Type      `bun:"type,notnull,default:'photo'" json:"-"`
	URL           string    `bun:"url,notnull" json:"url"`
}

// Tag is global; the legacy notes subsystem has its own owner-scoped tags.
type Tag struct {
	bun.BaseModel `bun:"table:post_tags"`
	ID            uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Name          string    `bun:"name,notnull,unique" json:"name"`
	Title         string    `bun:"title,notnull" json:"title"`
}

type PostTag struct {
	bun.BaseModel `bun:"table:post_tag_links"`
	PostID        uuid.UUID `bun:"post_id,pk,type:uuid" json:"post_id"`
	TagID         uuid.UUID `bun:"tag_id,pk,type:uuid" json:"tag_id"`
	Tag           *Tag      `bun:"rel:belongs-to,join:tag_id=id" json:"tag,omitempty"`
}
