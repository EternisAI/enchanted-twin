// Owner: august@eternis.ai
// Package tts implements text to speech functionality used by the frontend to generate audio from text.
package tts

import (
	"context"
	"io"
	"net/http"

	"github.com/EternisAI/enchanted-twin/pkg/tts/internal/model"
	"github.com/charmbracelet/log"

	"github.com/gorilla/websocket"
)

// TTSProvider is responsible for providing a stream of audio from a given text.
// This should be compatible to future implementation of full voice conversation pipeline.
type TTSProvider interface {
	Stream(ctx context.Context, text string) (io.ReadCloser, error)
}

// Service is a wrapper around the TTSProvider that provides a websocket endpoint for streaming audio.
type Service struct {
	addr     string
	provider TTSProvider
	upgrader websocket.Upgrader
	logger   log.Logger
}

func New(addr string, p TTSProvider, logger log.Logger) *Service {
	return &Service{
		addr:     addr,
		provider: p,
		logger:   logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

func (s *Service) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWS)

	srv := &http.Server{Addr: s.addr, Handler: mux}

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	s.logger.Info("Started TTS service on", "host", "ws://localhost"+s.addr+"/ws")
	return srv.ListenAndServe()
}

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
