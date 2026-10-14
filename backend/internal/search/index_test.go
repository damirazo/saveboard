package search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	modelconfig "saveboard/config"
	"strings"
	"testing"
)

func TestChunks(t *testing.T) {
	text := strings.Repeat("Русский текст с пробелами и emoji 🌱. ", 300)
	chunks := Chunks(text)
	if len(chunks) < 2 {
		t.Fatal("not chunked")
	}
	rebuilt := chunks[0]
	for i, c := range chunks {
		if len([]rune(c)) > 1600 {
			t.Fatal("oversized chunk")
		}
		if i > 0 {
			if !strings.HasSuffix(chunks[i-1], string([]rune(c)[:200])) {
				t.Fatal("lost overlap")
			}
			rebuilt += string([]rune(c)[200:])
		}
	}
	if rebuilt != strings.TrimSpace(text) {
		t.Fatal("lost content")
	}
	if PointID("generation", 0) != PointID("generation", 0) || PointID("generation", 0) == PointID("other", 0) {
		t.Fatal("point identity")
	}
}
func TestEmbeddingValidation(t *testing.T) {
	m := modelconfig.Model()
	wrong := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{map[string]string{"name": m.LocalName, "digest": m.Digest}}})
			return
		}
		var input struct {
			Truncate bool
			Input    []string
			Model    string
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		if input.Truncate || input.Model != m.LocalName {
			t.Error("invalid embedding options")
		}
		n := m.Dimensions
		if wrong {
			n--
		}
		v := make([]float32, n)
		v[0] = 1
		_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": [][]float32{v}})
	}))
	defer server.Close()
	client := Index{OllamaURL: server.URL}
	if err := client.VerifyModel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Embed(context.Background(), []string{"test"}); err != nil {
		t.Fatal(err)
	}
	wrong = true
	if _, err := client.Embed(context.Background(), []string{"test"}); err == nil {
		t.Fatal("accepted wrong dimension")
	}
}
