package main

import (
	"encoding/json"
	"os"
	"saveboard/internal/transport/httpapi"
)

func main() {
	_, api := httpapi.New(httpapi.Dependencies{})
	b, err := json.MarshalIndent(api.OpenAPI(), "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll("api", 0755); err != nil {
		panic(err)
	}
	if err := os.WriteFile("api/openapi.json", append(b, '\n'), 0644); err != nil {
		panic(err)
	}
}
