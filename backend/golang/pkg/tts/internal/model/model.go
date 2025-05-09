package model

import (
	"bytes"
	"encoding/json"
)

type Request struct {
	Model          string `json:"model"`
	Voice          string `json:"voice"`
	Input          string `json:"input"`
	ResponseFormat string `json:"response_format"`
	Stream         bool   `json:"stream"`
}

func (k Request) Encode() *bytes.Reader {
	b, _ := json.Marshal(k)
	return bytes.NewReader(b)
}
