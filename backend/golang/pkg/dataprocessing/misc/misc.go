package misc

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
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

// WithChunkSize allows setting a custom chunk size.
func (s *Source) WithChunkSize(size int) *Source {
	s.chunkSize = size
	return s
}

func (s *Source) Name() string {
	return "misc"
}

// IsHumanReadableContent uses a language model to determine if the content is human-readable.
func (s *Source) IsHumanReadableContent(ctx context.Context, content string) (bool, error) {
	contentSample := content
	if len(content) > 500 {
		contentSample = content[:500]
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(fmt.Sprintf("Analyze this text sample and determine if it contains human-readable content (not binary data, not just code, not just random characters). Reply with ONLY 'yes' or 'no'.\n\nText sample: %s", contentSample)),
	}

	response, err := s.openAiService.Completions(ctx, messages, []openai.ChatCompletionToolParam{}, "gemma3:1b")
	if err != nil {
		return false, fmt.Errorf("failed to analyze content: %w", err)
	}

	responseText := strings.ToLower(strings.TrimSpace(response.Content))
	return strings.Contains(responseText, "yes"), nil
}

// ExtractContentTags uses a language model to extract relevant tags from the content.
func (s *Source) ExtractContentTags(ctx context.Context, content string) ([]string, error) {
	contentSample := content
	if len(content) > 1000 {
		contentSample = content[:1000]
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(fmt.Sprintf("Extract 3-5 tags that describe this content. Reply with ONLY a comma-separated list of tags (no explanations, just the tags).\n\nText sample: %s", contentSample)),
	}

	response, err := s.openAiService.Completions(ctx, messages, []openai.ChatCompletionToolParam{}, "gemma3:1b")
	if err != nil {
		return nil, fmt.Errorf("failed to extract tags: %w", err)
	}

	// Process the response to extract tags
	responseText := strings.TrimSpace(response.Content)

	// Clean up line breaks, extra spaces, and potential markdown formatting
	responseText = strings.ReplaceAll(responseText, "\n", " ")
	responseText = strings.ReplaceAll(responseText, "  ", " ")
	responseText = strings.ReplaceAll(responseText, "* ", "")
	responseText = strings.ReplaceAll(responseText, "- ", "")

	// Split by commas
	tagsList := strings.Split(responseText, ",")

	// Clean up individual tags
	tags := make([]string, 0, len(tagsList))
	for _, tag := range tagsList {
		tag = strings.TrimSpace(tag)
		// Skip if empty or too long
		if tag != "" && len(tag) < 50 {
			tags = append(tags, tag)
		}
	}

	return tags, nil
}

func (s *Source) ProcessFile(filePath string) ([]types.Record, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("failed to close file %s: %v\n", filePath, err)
		}
	}()

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

// ProcessDirectory walks through a directory and processes all text files.
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
