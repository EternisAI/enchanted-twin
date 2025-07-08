package misc

import (
	"archive/zip"
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/KSpaceer/goppt"
	"github.com/charmbracelet/log"
	"github.com/guylaor/goword"
	"github.com/ledongthuc/pdf"
	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

const (
	DefaultChunkSize = 5000
)

type TextDocumentProcessor struct {
	openAiService    *ai.Service
	chunkSize        int
	completionsModel string
	store            *db.Store
	logger           *log.Logger
}

func NewTextDocumentProcessor(openAiService *ai.Service, completionsModel string, store *db.Store, logger *log.Logger) (*TextDocumentProcessor, error) {
	if openAiService == nil {
		return nil, fmt.Errorf("openAiService is nil")
	}

	if completionsModel == "" {
		return nil, fmt.Errorf("completionsModel is empty")
	}

	if store == nil {
		return nil, fmt.Errorf("store is nil")
	}

	if logger == nil {
		return nil, fmt.Errorf("logger is nil")
	}

	return &TextDocumentProcessor{
		openAiService:    openAiService,
		chunkSize:        DefaultChunkSize,
		completionsModel: completionsModel,
		store:            store,
		logger:           logger,
	}, nil
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
			s.logger.Warn("failed to close PDF file", "filePath", filePath, "error", err)
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

// ExtractTextFromWord extracts text content from a Word (.docx) file.
func (s *TextDocumentProcessor) ExtractTextFromWord(filePath string) (string, error) {
	text, err := goword.ParseText(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to extract text from Word file %s: %w", filePath, err)
	}
	return text, nil
}

// ExtractTextFromPPT extracts text content from a legacy PowerPoint (.ppt) file.
func (s *TextDocumentProcessor) ExtractTextFromPPT(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open PPT file %s: %w", filePath, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			s.logger.Warn("failed to close PPT file", "filePath", filePath, "error", err)
		}
	}()

	text, err := goppt.ExtractText(file)
	if err != nil {
		return "", fmt.Errorf("failed to extract text from PPT file %s: %w", filePath, err)
	}
	return text, nil
}

// ExtractTextFromPPTX extracts text content from a modern PowerPoint (.pptx) file.
func (s *TextDocumentProcessor) ExtractTextFromPPTX(filePath string) (string, error) {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open PPTX file %s: %w", filePath, err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			s.logger.Warn("failed to close PPTX reader", "filePath", filePath, "error", err)
		}
	}()

	var textBuilder strings.Builder

	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, "ppt/slides/slide") && strings.HasSuffix(file.Name, ".xml") {
			text, err := s.extractTextFromSlideXML(file)
			if err != nil {
				s.logger.Warn("Failed to extract text from slide", "file", file.Name, "error", err)
				continue
			}
			if text != "" {
				textBuilder.WriteString(text)
				textBuilder.WriteString("\n\n")
			}
		}
	}

	return textBuilder.String(), nil
}

// extractTextFromSlideXML extracts text from a slide XML file within the PPTX archive.
func (s *TextDocumentProcessor) extractTextFromSlideXML(file *zip.File) (string, error) {
	rc, err := file.Open()
	if err != nil {
		return "", err
	}
	defer func() {
		if err := rc.Close(); err != nil {
			s.logger.Warn("failed to close slide XML reader", "file", file.Name, "error", err)
		}
	}()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}

	xmlContent := string(data)

	return s.extractTextFromXMLFallback(xmlContent), nil
}

// extractTextFromXMLFallback is a fallback method to extract text from XML using string manipulation.
func (s *TextDocumentProcessor) extractTextFromXMLFallback(xmlContent string) string {
	var textBuilder strings.Builder
	extractedTexts := make(map[string]bool)

	start := 0
	for {
		startTag := strings.Index(xmlContent[start:], "<a:t>")
		if startTag == -1 {
			break
		}
		startTag += start + 5

		endTag := strings.Index(xmlContent[startTag:], "</a:t>")
		if endTag == -1 {
			break
		}
		endTag += startTag

		text := strings.TrimSpace(xmlContent[startTag:endTag])
		if text != "" && !extractedTexts[text] {
			textBuilder.WriteString(text)
			textBuilder.WriteString(" ")
			extractedTexts[text] = true
		}

		start = endTag + 6
	}

	otherTags := []string{
		"<p:t>", "</p:t>",
		"<a:p>", "</a:p>",
		"<p:p>", "</p:p>",
	}

	for i := 0; i < len(otherTags); i += 2 {
		openTag := otherTags[i]
		closeTag := otherTags[i+1]

		start := 0
		for {
			startTag := strings.Index(xmlContent[start:], openTag)
			if startTag == -1 {
				break
			}
			startTag += start + len(openTag)

			endTag := strings.Index(xmlContent[startTag:], closeTag)
			if endTag == -1 {
				break
			}
			endTag += startTag

			content := xmlContent[startTag:endTag]
			text := s.removeXMLTags(content)
			text = strings.TrimSpace(text)

			if text != "" && !extractedTexts[text] {
				textBuilder.WriteString(text)
				textBuilder.WriteString(" ")
				extractedTexts[text] = true
			}

			start = endTag + len(closeTag)
		}
	}

	result := strings.TrimSpace(textBuilder.String())
	return result
}

// removeXMLTags removes XML tags from text content.
func (s *TextDocumentProcessor) removeXMLTags(content string) string {
	var result strings.Builder
	inTag := false

	for _, char := range content {
		if char == '<' {
			inTag = true
		} else if char == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(char)
		}
	}

	return result.String()
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

func (s *TextDocumentProcessor) ProcessFile(ctx context.Context, filePath string) ([]memory.TextDocument, error) {
	// Check if the input is a directory
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info for %s: %w", filePath, err)
	}

	if info.IsDir() {
		// Handle directory by walking through all files
		return s.processDirectory(ctx, filePath)
	}

	// Handle single file
	return s.processSingleFile(ctx, filePath)
}

func (s *TextDocumentProcessor) processDirectory(ctx context.Context, dirPath string) ([]memory.TextDocument, error) {
	var allDocuments []memory.TextDocument

	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		documents, err := s.processSingleFile(ctx, path)
		if err != nil {
			s.logger.Warn("Failed to process file", "path", path, "error", err)
			return nil // Continue processing other files
		}

		allDocuments = append(allDocuments, documents...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking directory %s: %w", dirPath, err)
	}

	return allDocuments, nil
}

func (s *TextDocumentProcessor) processSingleFile(ctx context.Context, filePath string) ([]memory.TextDocument, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			s.logger.Warn("failed to close file", "filePath", filePath, "error", err)
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

	ext := strings.ToLower(filepath.Ext(fileName))
	isPdf := ext == ".pdf"
	isDocx := ext == ".docx"
	isPpt := ext == ".ppt"
	isPptx := ext == ".pptx"

	var textContent string

	if isPdf {
		s.logger.Info("Extracting text from PDF", "filePath", filePath)
		extractedText, err := s.ExtractTextFromPDF(filePath)
		if err != nil {
			return nil, err
		}
		textContent = extractedText
	} else if isDocx {
		extractedText, err := s.ExtractTextFromWord(filePath)
		if err != nil {
			return nil, err
		}
		textContent = extractedText
	} else if isPpt {
		extractedText, err := s.ExtractTextFromPPT(filePath)
		if err != nil {
			return nil, err
		}
		textContent = extractedText
	} else if isPptx {
		extractedText, err := s.ExtractTextFromPPTX(filePath)
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

		if !isPdf && !isDocx && !isPpt && !isPptx {
			isHumanReadable, err := s.IsHumanReadableContent(context.Background(), textContent)
			if err != nil {
				return nil, fmt.Errorf("error analyzing file %s: %w", filePath, err)
			}

			s.logger.Info("File", "fileName", fileName, "isHumanReadable", isHumanReadable)

			if !isHumanReadable {
				return nil, fmt.Errorf("file %s does not contain human-readable content", filePath)
			}
		}
	}

	s.logger.Info("Processing file", "fileName", fileName, "contentLength", len(textContent), "preview", textContent[:min(200, len(textContent))])

	if len(textContent) == 0 {
		emptyDoc := memory.TextDocument{
			FieldID:        fmt.Sprintf("misc-%s-%d", fileName, timestamp.Unix()),
			FieldContent:   "",
			FieldTimestamp: &timestamp,
			FieldSource:    s.Name(),
			FieldTags:      []string{},
			FieldMetadata: map[string]string{
				"filename": fileName,
				"path":     filePath,
				"type":     s.getFileType(ext),
			},
			FieldFilePath: filePath,
		}
		return []memory.TextDocument{emptyDoc}, nil
	}

	tags, err := s.ExtractContentTags(context.Background(), textContent)
	if err != nil {
		s.logger.Warn("Failed to extract tags", "filePath", filePath, "error", err)
		tags = []string{}
	}

	var documents []memory.TextDocument
	for i := 0; i < len(textContent); i += s.chunkSize {
		end := i + s.chunkSize
		if end > len(textContent) {
			end = len(textContent)
		}

		chunk := textContent[i:end]
		chunkIndex := i / s.chunkSize

		doc := memory.TextDocument{
			FieldID:        fmt.Sprintf("misc-%s-%d-chunk%d", fileName, timestamp.Unix(), chunkIndex),
			FieldContent:   chunk,
			FieldTimestamp: &timestamp,
			FieldSource:    s.Name(),
			FieldTags:      tags,
			FieldMetadata: map[string]string{
				"filename": fileName,
				"path":     filePath,
				"type":     s.getFileType(ext),
				"chunk":    fmt.Sprintf("%d", chunkIndex),
			},
			FieldFilePath: filePath,
		}
		documents = append(documents, doc)

		s.logger.Info("Created document", "fileName", fileName, "chunk", chunkIndex, "contentLength", len(chunk))
	}

	s.logger.Info("File processed into documents", "fileName", fileName, "documents", len(documents))
	return documents, nil
}

func (s *TextDocumentProcessor) getFileType(ext string) string {
	switch ext {
	case ".pdf":
		return "pdf"
	case ".docx":
		return "docx"
	case ".ppt":
		return "ppt"
	case ".pptx":
		return "pptx"
	default:
		return "text"
	}
}
