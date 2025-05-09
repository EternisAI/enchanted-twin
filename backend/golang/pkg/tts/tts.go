package tts

import (
	"context"
	"io"
)

type TTSProvider interface {
	Stream(ctx context.Context, text string) (io.ReadCloser, error)
}
