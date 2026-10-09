package posts

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestMarkdownSafety(t *testing.T) {
	for _, s := range []string{"<script>alert(1)</script>", "[x](javascript:alert%281%29)", "[x](data:text/html,x)", "![x](https://example.com/a.png)", "<img src=x onerror=alert(1)>", "[x](jav&#x61;script:alert(1))"} {
		if ValidateMarkdown(s) == nil {
			t.Errorf("accepted %q", s)
		}
	}
	for _, s := range []string{"# Заголовок\n\n**Текст**", "[сайт](https://example.com)", "```html\n<script>example</script>\n```"} {
		if e := ValidateMarkdown(s); e != nil {
			t.Errorf("rejected %q: %v", s, e)
		}
	}
}
func TestSlug(t *testing.T) {
	if got := Slug("Привет, мир! Ёж и щука"); got != "privet-mir-yozh-i-shchuka" {
		t.Fatal(got)
	}
}
func TestPhotoValidation(t *testing.T) {
	for _, b := range [][]byte{[]byte("<svg onload='alert(1)'/>"), []byte("GIF89a broken"), make([]byte, (10<<20)+1)} {
		if _, _, e := NormalizePhoto(b); e == nil {
			t.Fatal("accepted invalid image")
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	b.WriteString("<script>bad</script>")
	data, mime, e := NormalizePhoto(b.Bytes())
	if e != nil || mime != "image/png" || bytes.Contains(data, []byte("script")) {
		t.Fatalf("normalization failed: %s %v", mime, e)
	}
}
