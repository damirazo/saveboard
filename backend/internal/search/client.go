package search

import "github.com/qdrant/go-client/qdrant"

// Qdrant is a derived index. Future retrieval must re-check PostgreSQL ACL and
// active generation before returning any candidate text.
func NewClient(host, apiKey string) (*qdrant.Client, error) {
	return qdrant.NewClient(&qdrant.Config{Host: host, Port: 6334, APIKey: apiKey})
}
