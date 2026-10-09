package posts

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

var ErrInvalid = errors.New("invalid post")
var ErrNotFound = errors.New("post not found")
var ErrForbidden = errors.New("active owner required")

func invalid(s string) error { return fmt.Errorf("%w: %s", ErrInvalid, s) }

func safeURL(s string) bool {
	u, e := url.Parse(s)
	return e == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil
}

// Parse Markdown rather than scanning raw strings: code examples remain harmless text.
func ValidateMarkdown(s string) error {
	if !utf8.ValidString(s) || len(s) > 100000 || strings.TrimSpace(s) == "" || strings.ContainsRune(s, 0) {
		return invalid("Текст обязателен, максимум 100 КБ")
	}
	source := []byte(s)
	doc := goldmark.New().Parser().Parse(text.NewReader(source))
	return ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.RawHTML, *ast.HTMLBlock:
			return ast.WalkStop, invalid("HTML запрещён; используйте Markdown")
		case *ast.Image:
			return ast.WalkStop, invalid("Изображения добавляются в фото-пост")
		case *ast.Link:
			if !safeURL(string(v.Destination)) {
				return ast.WalkStop, invalid("Разрешены только HTTP(S) ссылки")
			}
		case *ast.AutoLink:
			if !safeURL(string(v.URL(source))) {
				return ast.WalkStop, invalid("Разрешены только HTTP(S) ссылки")
			}
		}
		return ast.WalkContinue, nil
	})
}

func Slug(title string) string {
	const ru = "абвгдеёжзийклмнопрстуфхцчшщъыьэюя"
	roman := []string{"a", "b", "v", "g", "d", "e", "yo", "zh", "z", "i", "y", "k", "l", "m", "n", "o", "p", "r", "s", "t", "u", "f", "kh", "ts", "ch", "sh", "shch", "", "y", "", "e", "yu", "ya"}
	mapping := map[rune]string{}
	for i, r := range []rune(ru) {
		mapping[r] = roman[i]
	}
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		if s, ok := mapping[r]; ok {
			b.WriteString(s)
		} else if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Join(strings.FieldsFunc(b.String(), func(r rune) bool { return r == '-' }), "-")
}
func validLabel(s string, max int) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= max && strings.TrimSpace(s) != "" && !strings.ContainsAny(s, "<>") && strings.IndexFunc(s, unicode.IsControl) < 0
}

// Re-encoding removes metadata and appended content. Animated GIFs use their first frame.
func NormalizePhoto(data []byte) ([]byte, string, error) {
	if len(data) == 0 || len(data) > 10<<20 {
		return nil, "", invalid("Фото: максимум 10 МБ")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 10000 || cfg.Height > 10000 || int64(cfg.Width)*int64(cfg.Height) > 20000000 {
		return nil, "", invalid("Неверное изображение или размер более 20 Мп")
	}
	switch format {
	case "jpeg", "png", "gif", "bmp", "webp":
	default:
		return nil, "", invalid("Неподдерживаемое изображение")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", invalid("Повреждённое изображение")
	}
	var out bytes.Buffer
	mime := "image/png"
	if format == "jpeg" {
		mime = "image/jpeg"
		err = jpeg.Encode(&out, img, &jpeg.Options{Quality: 90})
	} else {
		err = png.Encode(&out, img)
	}
	if err != nil || out.Len() > 10<<20 {
		return nil, "", invalid("Нормализованное фото превышает 10 МБ")
	}
	return out.Bytes(), mime, nil
}
