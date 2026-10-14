package config

import (
	_ "embed"
	"encoding/json"
)

//go:embed embedding-model.json
var modelJSON []byte

type EmbeddingModel struct {
	LocalName  string `json:"localName"`
	Digest     string `json:"digest"`
	Dimensions int    `json:"dimensions"`
}

func Model() EmbeddingModel {
	var m EmbeddingModel
	if err := json.Unmarshal(modelJSON, &m); err != nil {
		panic(err)
	}
	return m
}
