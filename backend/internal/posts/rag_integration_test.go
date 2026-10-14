//go:build integration

package posts

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"net/http"
	"net/http/httptest"
	modelconfig "saveboard/config"
	"saveboard/internal/jobs"
	platformdb "saveboard/internal/platform/db"
	"saveboard/internal/search"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRAGLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	container, err := postgres.Run(ctx, "postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336", postgres.WithDatabase("rag_test"), postgres.WithUsername("test"), postgres.WithPassword("test"), postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := platformdb.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = platformdb.Migrate(ctx, db.DB); err != nil {
		t.Fatal(err)
	}
	owner := uuid.NewString()
	if _, err = db.DB.ExecContext(ctx, `INSERT INTO users(id,username,display_name,status) VALUES($1,'rag-owner','RAG owner','active')`, owner); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	points := map[string]search.Point{}
	extractions := 0
	failUpsert := false
	failDelete := false
	var during func()
	model := modelconfig.Model()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.URL.Path == "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{map[string]string{"name": model.LocalName, "digest": model.Digest}}})
		case r.URL.Path == "/v1/extract":
			extractions++
			_ = json.NewEncoder(w).Encode(map[string]string{"title": "Article", "text": strings.Repeat("Осмысленный текст статьи для семантического поиска. ", 100)})
		case r.URL.Path == "/api/embed":
			var in struct{ Input []string }
			_ = json.NewDecoder(r.Body).Decode(&in)
			vectors := make([][]float32, len(in.Input))
			for i := range vectors {
				vectors[i] = make([]float32, 1024)
				vectors[i][0] = 1
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": vectors})
		case strings.HasSuffix(r.URL.Path, "/points/delete"):
			if failDelete {
				w.WriteHeader(503)
				return
			}
			var body struct {
				Filter struct {
					Must []struct{ Match struct{ Value string } }
				}
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			for id, p := range points {
				if p.Payload["generation_id"] == body.Filter.Must[0].Match.Value {
					delete(points, id)
				}
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case strings.HasSuffix(r.URL.Path, "/points"):
			var in struct{ Points []search.Point }
			_ = json.NewDecoder(r.Body).Decode(&in)
			for _, p := range in.Points {
				points[p.ID] = p
			}
			if during != nil {
				during()
			}
			if failUpsert {
				w.WriteHeader(503)
				return
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"config": map[string]any{"params": map[string]any{"vectors": map[string]any{"size": 1024, "distance": "Cosine"}}}}})
		}
	}))
	defer server.Close()
	service := NewService(db.DB)
	index := &search.Index{OllamaURL: server.URL, QdrantURL: server.URL}
	worker := jobs.RAGWorker{DB: db.DB, Index: index, URL: server.URL, Token: "test"}
	write := Write{Title: "RAG", Type: "link", URL: "https://example.com/a", Visibility: "private", Tags: []TagInput{{Title: "RAG"}}}
	post, err := service.Save(ctx, owner, "", write)
	if err != nil {
		t.Fatal(err)
	}
	if post.IndexRAG || post.RAGState != "" {
		t.Fatal("default should be off")
	}
	write.IndexRAG = true
	save := func() View {
		t.Helper()
		p, e := service.Save(ctx, owner, post.ID, write)
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	revision := func() string {
		t.Helper()
		var id string
		if e := db.DB.QueryRowContext(ctx, `SELECT revision FROM post_rag WHERE post_id=$1`, post.ID).Scan(&id); e != nil {
			t.Fatal(e)
		}
		return id
	}
	run := func(id string, attempt int) error {
		return worker.Work(ctx, &river.Job[jobs.RAGArgs]{JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: 3}, Args: jobs.RAGArgs{PostID: post.ID, Revision: id}})
	}
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if e := db.DB.QueryRowContext(ctx, query, args...).Scan(&n); e != nil {
			t.Fatal(e)
		}
		return n
	}
	reconcile := func() error {
		producer, e := jobs.Producer(db.DB)
		if e != nil {
			return e
		}
		return jobs.ReconcileRAG(ctx, db.DB, index, func(c context.Context, id int64) error { _, e := producer.JobCancel(c, id); return e })
	}
	if p := save(); !p.IndexRAG || p.RAGState != "pending" || p.CaptureState != "" {
		t.Fatalf("independent RAG %+v", p)
	}
	rev := revision()
	failUpsert = true
	if e := run(rev, 1); e == nil {
		t.Fatal("expected ambiguous upsert failure")
	}
	if count(`SELECT count(*) FROM notifications WHERE event_key=$1`, "rag:"+rev) != 0 {
		t.Fatal("premature notification")
	}
	failUpsert = false
	if e := run(rev, 2); e != nil {
		t.Fatal(e)
	}
	if e := run(rev, 3); e != nil {
		t.Fatal(e)
	}
	if extractions != 1 {
		t.Fatal("retry extracted again")
	}
	if count(`SELECT count(*) FROM notifications WHERE event_key=$1 AND importance='normal'`, "rag:"+rev) != 1 {
		t.Fatal("notification not idempotent")
	}
	if count(`SELECT jsonb_array_length(chunks) FROM rag_documents WHERE id=$1`, rev) != len(points) {
		t.Fatal("duplicated chunks")
	}
	write.Title = "Renamed"
	save()
	if revision() != rev {
		t.Fatal("metadata edit reindexed")
	}
	// Editing during upsert cannot publish the old generation; its points are purged.
	write.URL = "https://example.com/b"
	save()
	stale := revision()
	during = func() {
		write.URL = "https://example.com/c"
		_, e := service.Save(ctx, owner, post.ID, write)
		if e != nil {
			t.Error(e)
		}
	}
	if e := run(stale, 1); e != nil {
		t.Fatal(e)
	}
	during = nil
	if count(`SELECT count(*) FROM notifications WHERE event_key=$1`, "rag:"+stale) != 0 {
		t.Fatal("stale success")
	}
	if e := reconcile(); e != nil {
		t.Fatal(e)
	}
	mu.Lock()
	for _, p := range points {
		if p.Payload["generation_id"] == stale || p.Payload["generation_id"] == rev {
			t.Error("stale points retained")
		}
	}
	mu.Unlock()
	current := revision()
	if e := run(current, 1); e != nil {
		t.Fatal(e)
	}
	// Disabling is local/transactional even if Qdrant is down; cleanup is retried.
	write.IndexRAG = false
	save()
	failDelete = true
	if e := reconcile(); e == nil {
		t.Fatal("expected cleanup outage")
	}
	failDelete = false
	if e := reconcile(); e != nil {
		t.Fatal(e)
	}
	if len(points) != 0 {
		t.Fatal("disabled index retained")
	}
	// Expiry and a lost final attempt emit exactly one critical notification.
	write.IndexRAG = true
	save()
	expired := revision()
	if _, err = db.DB.ExecContext(ctx, `UPDATE rag_documents SET deadline_at=now()-interval '1 second' WHERE id=$1`, expired); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if e := reconcile(); e != nil {
			t.Fatal(e)
		}
	}
	if count(`SELECT count(*) FROM notifications WHERE event_key=$1 AND importance='critical'`, "rag:"+expired) != 1 {
		t.Fatal("expiry notification")
	}
	save()
	lost := revision()
	if _, err = db.DB.ExecContext(ctx, `UPDATE river_job SET state='discarded',finalized_at=now() WHERE args->>'revision'=$1`, lost); err != nil {
		t.Fatal(err)
	}
	if e := reconcile(); e != nil {
		t.Fatal(e)
	}
	if count(`SELECT count(*) FROM notifications WHERE event_key=$1 AND importance='critical'`, "rag:"+lost) != 1 {
		t.Fatal("lost attempt notification")
	}
	save()
	last := revision()
	failUpsert = true
	if e := run(last, 3); e == nil {
		t.Fatal("expected final failure")
	}
	failUpsert = false
	if count(`SELECT count(*) FROM notifications WHERE event_key=$1 AND importance='critical'`, "rag:"+last) != 1 {
		t.Fatal("final failure notification")
	}
	save()
	if e := run(revision(), 1); e != nil {
		t.Fatal(e)
	}
	if e := service.Draft(ctx, owner, post.ID); e != nil {
		t.Fatal(e)
	}
	if e := reconcile(); e != nil {
		t.Fatal(e)
	}
	if len(points) != 0 {
		t.Fatal("draft index retained")
	}
}
