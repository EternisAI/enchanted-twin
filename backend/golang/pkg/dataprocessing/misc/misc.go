package misc

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

const (
	DefaultChunkSize = 200
)

type Source struct {
	openAiService *ai.Service
	chunkSize     int
}

func New(openAiService *ai.Service) *Source {
	return &Source{
		openAiService: openAiService,
		chunkSize:     DefaultChunkSize,
	}
}

// WithChunkSize allows setting a custom chunk size
func (s *Source) WithChunkSize(size int) *Source {
	s.chunkSize = size
	return s
}

func (s *Source) Name() string {
	return "misc"
}

// IsHumanReadableContent uses OpenAI to determine if the content is human-readable
func (s *Source) IsHumanReadableContent(ctx context.Context, content string) (bool, error) {
	// Create content sample to analyze (use a limited sample to save tokens)
	contentSample := content
	if len(content) > 500 {
		contentSample = content[:500]
	}

	// Define a tool for checking if content is human-readable
	isHumanReadableTool := openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name: "is_human_readable",
			Description: param.NewOpt(
				"Determine if the provided text sample contains human-readable content",
			),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"textSample": map[string]interface{}{
						"type":        "string",
						"description": "The text sample to analyze",
					},
					"isHumanReadable": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether the text contains human-readable content (not binary data, not just code, not just random characters)",
					},
				},
				"required": []string{"textSample", "isHumanReadable"},
			},
		},
	}

	// Create a simple user message with our request
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(fmt.Sprintf("Please analyze this text sample and determine if it contains human-readable content: %s", contentSample)),
	}

	// Run completion with our tool
	response, err := s.openAiService.Completions(ctx, messages, []openai.ChatCompletionToolParam{isHumanReadableTool}, "gpt-4o-mini")
	if err != nil {
		return false, fmt.Errorf("failed to analyze content: %w", err)
	}

	// Parse the tool response
	if response.ToolCalls != nil && len(response.ToolCalls) > 0 {
		toolCall := response.ToolCalls[0]
		if toolCall.Function.Name == "is_human_readable" {
			// Extract the boolean value from the function arguments
			arguments := toolCall.Function.Arguments
			if strings.Contains(arguments, "\"isHumanReadable\":true") {
				return true, nil
			} else if strings.Contains(arguments, "\"isHumanReadable\":false") {
				return false, nil
			} else {
				return false, fmt.Errorf("unexpected tool response format: %s", arguments)
			}
		}
	}

	// Fallback to simple text response check
	return strings.Contains(strings.ToLower(response.Content), "true"), nil
}

func (s *Source) ProcessFile(filePath string) ([]types.Record, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info for %s: %w", filePath, err)
	}

	timestamp := fileInfo.ModTime()
	fileName := fileInfo.Name()

	scanner := bufio.NewScanner(file)
	var fullContent strings.Builder

	firstLine := true
	for scanner.Scan() {
		if !firstLine {
			fullContent.WriteString("\n")
		}
		fullContent.WriteString(scanner.Text())
		firstLine = false
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	textContent := fullContent.String()

	// Return early if the file is empty
	if len(textContent) == 0 {
		emptyRecord := types.Record{
			Data: map[string]interface{}{
				"content":  "",
				"filename": fileName,
				"type":     "text",
				"path":     filePath,
			},
			Timestamp: timestamp,
			Source:    s.Name(),
		}
		return []types.Record{emptyRecord}, nil
	}

	isHumanReadable, err := s.IsHumanReadableContent(context.Background(), textContent[min(len(textContent), 1000):])
	fmt.Printf("filePath: %s\n", filePath)
	fmt.Printf("isHumanReadable: %t\n", isHumanReadable)
	fmt.Printf("textContent: %s\n", textContent)

	if err != nil {
		return nil, fmt.Errorf("error analyzing file %s: %w", filePath, err)
	}

	if !isHumanReadable {
		return nil, fmt.Errorf("file %s does not contain human-readable content", filePath)
	}

	var records []types.Record

	// Split by lines and create a record for each line
	lines := strings.Split(textContent, "\n")
	for i, line := range lines {
		records = append(records, types.Record{
			Data: map[string]interface{}{
				"content":  line,
				"filename": fileName,
				"chunk":    i,
				"line":     i + 1,
			},
			Timestamp: timestamp,
			Source:    s.Name(),
		})
	}

	return records, nil
}

// ProcessDirectory walks through a directory and processes all text files
func (s *Source) ProcessDirectory(inputPath string) ([]types.Record, error) {
	var allRecords []types.Record

	err := filepath.WalkDir(inputPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		records, err := s.ProcessFile(path)
		if err != nil {
			fmt.Printf("Warning: Failed to process file %s: %v\n", path, err)
			return nil
		}

		allRecords = append(allRecords, records...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking directory %s: %w", inputPath, err)
	}

	fmt.Printf("Successfully processed %d records from directory %s\n", len(allRecords), inputPath)
	return allRecords, nil
}

func (s *Source) Sync(ctx context.Context, accessToken string) ([]types.Record, error) {
	return nil, fmt.Errorf("sync not supported for local text files")
}
