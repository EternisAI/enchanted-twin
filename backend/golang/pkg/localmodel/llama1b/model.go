package llama1b

import (
	"context"

	"github.com/EternisAI/enchanted-twin/pkg/localmodel"
)

var _ localmodel.CompletionModel = (*LlamaModel)(nil)

type LlamaModel struct {
}

func NewLlamaModel() *LlamaModel {
	return &LlamaModel{}
}

func (m *LlamaModel) Completions(ctx context.Context, input string, maxTokens int) ([]string, error) {
	return nil, nil
}
