package llama1b

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/localmodel"
)

var _ localmodel.Completion = (*LlamaModel)(nil)

type LlamaModel struct {
	binaryPath string
	modelDir   string
}

func NewLlamaModel(
	binaryPath string,
	modelDir string,
) *LlamaModel {
	return &LlamaModel{
		binaryPath: binaryPath,
		modelDir:   modelDir,
	}
}

func (m *LlamaModel) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	systemPrompt, userPrompt, err := m.extractPrompts(messages)
	if err != nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("failed to extract prompts: %w", err)
	}

	output, err := m.executeCLI(ctx, systemPrompt, userPrompt)
	if err != nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("failed to execute CLI: %w", err)
	}

	response := openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: output,
	}

	return response, nil
}

func (m *LlamaModel) extractPrompts(messages []openai.ChatCompletionMessageParamUnion) (string, string, error) {
	var systemPrompt, userPrompt string

	for _, msg := range messages {
		if msg.OfSystem.Content.OfString.Value != "" {
			systemPrompt = msg.OfSystem.Content.OfString.Value
		}
		if msg.OfUser.Content.OfString.Value != "" {
			userPrompt = msg.OfUser.Content.OfString.Value
		}
	}

	return systemPrompt, userPrompt, nil
}

func (m *LlamaModel) executeCLI(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "llama", "--system", systemPrompt, "--prompt", userPrompt)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("CLI execution failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
