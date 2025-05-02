package misc

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

const (
	DefaultChunkSize = 5000
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
	contentSample := content
	if len(content) > 500 {
		contentSample = content[:500]
	}

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

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(fmt.Sprintf("Please analyze this text sample and determine if it contains human-readable content: %s", contentSample)),
	}

	response, err := s.openAiService.Completions(ctx, messages, []openai.ChatCompletionToolParam{isHumanReadableTool}, "gpt-4o-mini")
	if err != nil {
		return false, fmt.Errorf("failed to analyze content: %w", err)
	}

	if response.ToolCalls != nil && len(response.ToolCalls) > 0 {
		toolCall := response.ToolCalls[0]
		if toolCall.Function.Name == "is_human_readable" {
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

	return strings.Contains(strings.ToLower(response.Content), "true"), nil
}

// ExtractContentTags uses OpenAI to extract relevant tags from the content
func (s *Source) ExtractContentTags(ctx context.Context, content string) ([]string, error) {
	contentSample := content
	if len(content) > 1000 {
		contentSample = content[:1000]
	}

	extractTagsTool := openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name: "extract_tags",
			Description: param.NewOpt(
				"Extract relevant tags that describe the content",
			),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"textSample": map[string]interface{}{
						"type":        "string",
						"description": "The text sample to analyze",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"description": "A list of 3-5 tags that describe the content's topic, domain, and key concepts",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required": []string{"textSample", "tags"},
			},
		},
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(fmt.Sprintf("Please analyze this text sample and extract 3-5 relevant tags that describe its content: %s", contentSample)),
	}

	response, err := s.openAiService.Completions(ctx, messages, []openai.ChatCompletionToolParam{extractTagsTool}, "gpt-4o-mini")
	if err != nil {
		return nil, fmt.Errorf("failed to extract tags: %w", err)
	}

	if response.ToolCalls != nil && len(response.ToolCalls) > 0 {
		toolCall := response.ToolCalls[0]
		if toolCall.Function.Name == "extract_tags" {
			arguments := toolCall.Function.Arguments

			if strings.Contains(arguments, "\"tags\":") {
				tagsStart := strings.Index(arguments, "\"tags\":")
				if tagsStart != -1 {
					tagsJSON := arguments[tagsStart:]
					arrayStart := strings.Index(tagsJSON, "[")
					arrayEnd := strings.Index(tagsJSON, "]")
					if arrayStart != -1 && arrayEnd != -1 && arrayEnd > arrayStart {
						tagsArray := tagsJSON[arrayStart+1 : arrayEnd]
						tagsParts := strings.Split(tagsArray, ",")
						tags := make([]string, 0, len(tagsParts))
						for _, tag := range tagsParts {
							tag = strings.TrimSpace(tag)
							tag = strings.Trim(tag, "\"")
							if tag != "" {
								tags = append(tags, tag)
							}
						}
						return tags, nil
					}
				}
			}
		}
	}

	return []string{}, nil
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
	var content strings.Builder

	firstLine := true
	for scanner.Scan() {
		if !firstLine {
			content.WriteString("\n")
		}
		content.WriteString(scanner.Text())
		firstLine = false
	}

	isHumanReadable, err := s.IsHumanReadableContent(context.Background(), content.String())
	fmt.Printf("filePath: %s\n", filePath)
	fmt.Printf("isHumanReadable: %t\n", isHumanReadable)
	fmt.Printf("textContent: %s\n", content.String())

	if err != nil {
		return nil, fmt.Errorf("error analyzing file %s: %w", filePath, err)
	}

	if !isHumanReadable {
		return nil, fmt.Errorf("file %s does not contain human-readable content", filePath)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	textContent := content.String()

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

	tags, err := s.ExtractContentTags(context.Background(), textContent)
	if err != nil {
		fmt.Printf("Warning: Failed to extract tags for %s: %v\n", filePath, err)
		tags = []string{}
	}

	var records []types.Record
	for i := 0; i < len(textContent); i += s.chunkSize {
		end := i + s.chunkSize
		if end > len(textContent) {
			end = len(textContent)
		}

		chunk := textContent[i:end]
		records = append(records, types.Record{
			Data: map[string]interface{}{
				"content":  chunk,
				"filename": fileName,
				"chunk":    i / s.chunkSize,
				"tags":     tags,
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

func ToDocuments(records []types.Record) ([]memory.TextDocument, error) {
	documents := make([]memory.TextDocument, 0, len(records))
	for _, record := range records {

		metadata := map[string]string{}

		content := ""
		if contentVal, ok := record.Data["content"]; ok && contentVal != nil {
			if contentStr, ok := contentVal.(string); ok {
				content = contentStr
			}
		}

		if pathVal, ok := record.Data["path"]; ok && pathVal != nil {
			if pathStr, ok := pathVal.(string); ok {
				metadata["path"] = pathStr
			}
		}

		var tags []string
		if tagsVal, ok := record.Data["tags"]; ok && tagsVal != nil {
			if tagsArr, ok := tagsVal.([]string); ok {
				tags = tagsArr
			}
		}

		documents = append(documents, memory.TextDocument{
			Content:   content,
			Timestamp: &record.Timestamp,
			Metadata:  metadata,
			Tags:      tags,
		})
	}
	return documents, nil
}
