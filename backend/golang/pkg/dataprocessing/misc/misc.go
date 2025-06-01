package misc

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
)

const (
	DefaultChunkSize = 5000
)

type TextDocumentProcessor struct {
	openAiService    *ai.Service
	chunkSize        int
	completionsModel string
}

func NewTextDocumentProcessor(openAiService *ai.Service, completionsModel string) *TextDocumentProcessor {
	return &TextDocumentProcessor{
		openAiService:    openAiService,
		chunkSize:        DefaultChunkSize,
		completionsModel: completionsModel,
	}
}

func (s *TextDocumentProcessor) Name() string {
	return "misc"
}

// IsHumanReadableContent determines if the content is human-readable text.
func (s *TextDocumentProcessor) IsHumanReadableContent(ctx context.Context, content string) (bool, error) {
	if len(content) == 0 {
		return true, nil
	}

	if len(content) >= 4 {
		firstFourBytes := content[:4]

		// PDF files are now handled separately
		if strings.HasPrefix(firstFourBytes, "%PDF") {
			return true, nil
		}

		if strings.HasPrefix(firstFourBytes, "PK\x03\x04") {
			return false, nil
		}

		if strings.HasPrefix(firstFourBytes, "\x89PNG") {
			return false, nil
		}

		if strings.HasPrefix(firstFourBytes, "GIF8") {
			return false, nil
		}

		if strings.HasPrefix(firstFourBytes, "\xff\xd8") {
			return false, nil
		}
	}

	var sampleContent string
	if len(content) > 3000 {
		beginning := content[:1000]
		middle := content[len(content)/2-500 : len(content)/2+500]
		end := content[len(content)-1000:]
		sampleContent = beginning + middle + end
	} else {
		sampleContent = content
	}

	var (
		totalChars        = 0
		printableChars    = 0
		wordChars         = 0
		replacementChars  = 0
		maxWordLength     = 0
		currentWordLength = 0
		zeroBytes         = 0
		highBitChars      = 0
		controlChars      = 0
		spaces            = 0
		punctuation       = 0
	)

	for _, r := range sampleContent {
		totalChars++

		if r == 0 {
			zeroBytes++
		}

		if r == '\uFFFD' {
			replacementChars++
		}

		if (r < 32 && r != '\n' && r != '\r' && r != '\t') || (r >= 127 && r < 160) {
			controlChars++
		}

		if r > 127 {
			highBitChars++
		}

		if r == ' ' {
			spaces++
		}

		if strings.ContainsRune(",.;:!?-\"'()[]{}", r) {
			punctuation++
		}

		if r >= 32 && r <= 126 {
			printableChars++

			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				wordChars++
				currentWordLength++
			} else {
				if currentWordLength > maxWordLength {
					maxWordLength = currentWordLength
				}
				currentWordLength = 0
			}
		} else if r != '\n' && r != '\r' && r != '\t' {
			if currentWordLength > maxWordLength {
				maxWordLength = currentWordLength
			}
			currentWordLength = 0
		}
	}

	if currentWordLength > maxWordLength {
		maxWordLength = currentWordLength
	}

	if totalChars == 0 {
		return false, nil
	}

	replacementRatio := float64(replacementChars) / float64(totalChars)
	printableRatio := float64(printableChars) / float64(totalChars)
	wordCharRatio := float64(wordChars) / float64(totalChars)
	zeroByteRatio := float64(zeroBytes) / float64(totalChars)
	controlCharRatio := float64(controlChars) / float64(totalChars)

	if zeroByteRatio > 0.01 {
		return false, nil
	}

	if replacementRatio > 0.05 {
		return false, nil
	}

	if controlCharRatio > 0.1 {
		return false, nil
	}

	if float64(highBitChars)/float64(totalChars) > 0.2 && printableRatio < 0.5 {
		return false, nil
	}

	if printableRatio < 0.7 {
		return false, nil
	}

	if wordCharRatio < 0.2 {
		return false, nil
	}

	if totalChars > 100 && spaces == 0 && punctuation == 0 {
		return false, nil
	}

	if maxWordLength < 3 {
		return false, nil
	}

	return true, nil
}

// ExtractTextFromPDF extracts text content from a PDF file.
func (s *TextDocumentProcessor) ExtractTextFromPDF(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open PDF file %s: %w", filePath, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Printf("failed to close PDF file %s: %v\n", filePath, err)
		}
	}()

	var textBuilder strings.Builder
	totalPage := r.NumPage()

	for pageIndex := 1; pageIndex <= totalPage; pageIndex++ {
		p := r.Page(pageIndex)
		if p.V.IsNull() {
			continue
		}

		text, err := p.GetPlainText(nil)
		if err != nil {
			return "", fmt.Errorf("failed to extract text from page %d of PDF file %s: %w", pageIndex, filePath, err)
		}
		textBuilder.WriteString(text)
		textBuilder.WriteString("\n\n")
	}

	return textBuilder.String(), nil
}

// ExtractContentTags uses a language model to extract relevant tags from the content.
func (s *TextDocumentProcessor) ExtractContentTags(ctx context.Context, content string) ([]string, error) {
	contentSample := content
	if len(content) > 1000 {
		contentSample = content[:1000]
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(fmt.Sprintf("Extract 3-5 tags that describe this content. Reply with ONLY a comma-separated list of tags (no explanations, just the tags).\n\nText sample: %s", contentSample)),
	}

	response, err := s.openAiService.Completions(ctx, messages, []openai.ChatCompletionToolParam{}, s.completionsModel)
	if err != nil {
		return nil, fmt.Errorf("failed to extract tags: %w", err)
	}

	responseText := strings.TrimSpace(response.Content)

	responseText = strings.ReplaceAll(responseText, "\n", " ")
	responseText = strings.ReplaceAll(responseText, "  ", " ")
	responseText = strings.ReplaceAll(responseText, "* ", "")
	responseText = strings.ReplaceAll(responseText, "- ", "")

	tagsList := strings.Split(responseText, ",")

	tags := make([]string, 0, len(tagsList))
	for _, tag := range tagsList {
		tag = strings.TrimSpace(tag)
		if tag != "" && len(tag) < 50 {
			tags = append(tags, tag)
		}
	}

	return tags, nil
}

func (s *TextDocumentProcessor) ProcessFile(filePath string) ([]types.Record, error) {
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

	const maxFileSize = 10 * 1024 * 1024
	if fileInfo.Size() > maxFileSize {
		return nil, fmt.Errorf("file %s is too large (%d bytes), maximum size is %d bytes", filePath, fileInfo.Size(), maxFileSize)
	}

	timestamp := fileInfo.ModTime()
	fileName := fileInfo.Name()

	isPdf := strings.ToLower(filepath.Ext(fileName)) == ".pdf"

	var textContent string

	if isPdf {
		extractedText, err := s.ExtractTextFromPDF(filePath)
		if err != nil {
			return nil, err
		}
		textContent = extractedText
	} else {
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

		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
		}

		textContent = content.String()

		if !isPdf {
			isHumanReadable, err := s.IsHumanReadableContent(context.Background(), textContent)
			if err != nil {
				return nil, fmt.Errorf("error analyzing file %s: %w", filePath, err)
			}

			fmt.Printf("isHumanReadable: %t\n", isHumanReadable)

			if !isHumanReadable {
				return nil, fmt.Errorf("file %s does not contain human-readable content", filePath)
			}
		}
	}

	if len(textContent) == 0 {
		emptyRecord := types.Record{
			Data: map[string]interface{}{
				"content":  "",
				"filename": fileName,
				"type": func() string {
					if isPdf {
						return "pdf"
					} else {
						return "text"
					}
				}(),
				"path": filePath,
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
				"type": func() string {
					if isPdf {
						return "pdf"
					} else {
						return "text"
					}
				}(),
				"chunk": i / s.chunkSize,
				"tags":  tags,
			},
			Timestamp: timestamp,
			Source:    s.Name(),
		})
	}

	return records, nil
}

func (s *TextDocumentProcessor) ProcessDirectory(inputPath string) ([]types.Record, error) {
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

func (s *TextDocumentProcessor) Sync(ctx context.Context, accessToken string) ([]types.Record, error) {
	return nil, fmt.Errorf("sync not supported for local text files")
}

func (s *TextDocumentProcessor) ToDocuments(records []types.Record) ([]memory.Document, error) {
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
			FieldContent:   content,
			FieldTimestamp: &record.Timestamp,
			FieldMetadata:  metadata,
			FieldTags:      tags,
		})
	}

	var documents_ []memory.Document
	for _, document := range documents {
		documents_ = append(documents_, &document)
	}

	return documents_, nil
}
