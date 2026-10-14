package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"math"
	"net/http"
	modelconfig "saveboard/config"
	"strings"
)

const Collection = "saveboard_bge_m3_790764642607_v1"

type Index struct{ OllamaURL, QdrantURL, APIKey string }
type Point struct {
	ID      string         `json:"id"`
	Vector  []float32      `json:"vector"`
	Payload map[string]any `json:"payload"`
}

func request(ctx context.Context, method, url, key string, body, out any) (int, error) {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return 0, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(data))
	if err != nil {
		return 0, errors.New("invalid service address")
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("api-key", key)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return 0, errors.New("index service unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return res.StatusCode, fmt.Errorf("index service HTTP %d", res.StatusCode)
	}
	if out != nil {
		raw, e := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		if e != nil {
			return res.StatusCode, e
		}
		if e = json.Unmarshal(raw, out); e != nil {
			return res.StatusCode, errors.New("invalid index response")
		}
	}
	return res.StatusCode, nil
}
func (i *Index) VerifyModel(ctx context.Context) error {
	var tags struct {
		Models []struct{ Name, Digest string } `json:"models"`
	}
	_, err := request(ctx, "GET", i.OllamaURL+"/api/tags", "", nil, &tags)
	if err != nil {
		return err
	}
	m := modelconfig.Model()
	for _, tag := range tags.Models {
		if tag.Name == m.LocalName && strings.TrimPrefix(tag.Digest, "sha256:") == strings.TrimPrefix(m.Digest, "sha256:") {
			return nil
		}
	}
	return errors.New("locked embedding model not installed")
}
func (i *Index) Embed(ctx context.Context, chunks []string) ([][]float32, error) {
	var out struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	m := modelconfig.Model()
	_, err := request(ctx, "POST", i.OllamaURL+"/api/embed", "", map[string]any{"model": m.LocalName, "input": chunks, "truncate": false}, &out)
	if err != nil {
		return nil, err
	}
	if len(out.Embeddings) != len(chunks) {
		return nil, errors.New("embedding count mismatch")
	}
	for _, v := range out.Embeddings {
		if len(v) != m.Dimensions {
			return nil, errors.New("embedding dimension mismatch")
		}
		var norm float64
		for _, x := range v {
			if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
				return nil, errors.New("invalid embedding")
			}
			norm += float64(x) * float64(x)
		}
		if norm == 0 {
			return nil, errors.New("zero embedding")
		}
	}
	return out.Embeddings, nil
}
func (i *Index) Ensure(ctx context.Context) error {
	var out struct {
		Result struct {
			Config struct {
				Params struct {
					Vectors struct {
						Size     int    `json:"size"`
						Distance string `json:"distance"`
					} `json:"vectors"`
				} `json:"params"`
			} `json:"config"`
		} `json:"result"`
	}
	path := i.QdrantURL + "/collections/" + Collection
	status, err := request(ctx, "GET", path, i.APIKey, nil, &out)
	if status == 404 {
		_, createErr := request(ctx, "PUT", path, i.APIKey, map[string]any{"vectors": map[string]any{"size": modelconfig.Model().Dimensions, "distance": "Cosine"}}, nil)
		// Another worker may create concurrently. Re-read and validate in either case.
		_, err = request(ctx, "GET", path, i.APIKey, nil, &out)
		if err != nil && createErr != nil {
			return createErr
		}
	}
	if err != nil {
		return err
	}
	if out.Result.Config.Params.Vectors.Size != modelconfig.Model().Dimensions || out.Result.Config.Params.Vectors.Distance != "Cosine" {
		return errors.New("incompatible Qdrant collection")
	}
	return nil
}
func (i *Index) Upsert(ctx context.Context, points []Point) error {
	_, err := request(ctx, "PUT", i.QdrantURL+"/collections/"+Collection+"/points?wait=true", i.APIKey, map[string]any{"points": points}, nil)
	return err
}
func (i *Index) Delete(ctx context.Context, revision string) error {
	status, err := request(ctx, "POST", i.QdrantURL+"/collections/"+Collection+"/points/delete?wait=true", i.APIKey, map[string]any{"filter": map[string]any{"must": []any{map[string]any{"key": "generation_id", "match": map[string]any{"value": revision}}}}}, nil)
	if status == 404 {
		return nil
	}
	return err
}
func PointID(revision string, ordinal int) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(revision+":"+fmt.Sprint(ordinal))).String()
}

// Chunk by Unicode code points; bounded overlap preserves nearby context.
func Chunks(text string) []string {
	r := []rune(strings.TrimSpace(text))
	var chunks []string
	for start := 0; start < len(r); {
		end := min(start+1600, len(r))
		if end < len(r) {
			for j := end; j > start+1200; j-- {
				if r[j-1] == '\n' || r[j-1] == ' ' {
					end = j
					break
				}
			}
		}
		chunks = append(chunks, string(r[start:end]))
		if end == len(r) {
			break
		}
		start = end - 200
	}
	return chunks
}
