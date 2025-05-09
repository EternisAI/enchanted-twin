package tts

import (
	"context"
	"io"
	"net/http"

	"github.com/EternisAI/enchanted-twin/pkg/tts/internal/model"
)

// Kokoro TTS model
// https://github.com/remsky/Kokoro-FastAPI
type Kokoro struct {
	Endpoint string
	Model    string
	Voice    string
}

func (k Kokoro) Stream(ctx context.Context, text string) (io.ReadCloser, error) {
	req := model.Request{
		Model:          k.Model,
		Voice:          k.Voice,
		Input:          text,
		ResponseFormat: "mp3",
		Stream:         true,
	}
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, k.Endpoint, req.Encode())
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
