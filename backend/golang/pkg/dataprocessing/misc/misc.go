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
	"time"

	"github.com/KSpaceer/goppt"
	"github.com/charmbracelet/log"
	"github.com/guylaor/goword"
	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/constants"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

const (
	DefaultChunkSize = 5000
)

type TextDocumentProcessor struct {
	chunkSize  int
	store      *db.Store
	logger     *log.Logger
	pdfiumPool pdfium.Pool
}

func NewTextDocumentProcessor(store *db.Store, logger *log.Logger) (*TextDocumentProcessor, error) {
	if store == nil {
		return nil, fmt.Errorf("store is nil")
	}

	if logger == nil {
		return nil, fmt.Errorf("logger is nil")
	}

	// Initialize PDFium pool
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  2,
		MaxTotal: 3,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PDFium pool: %w", err)
	}

	return &TextDocumentProcessor{
		chunkSize:  DefaultChunkSize,
		store:      store,
		logger:     logger,
		pdfiumPool: pool,
	}, nil
}

func (s *TextDocumentProcessor) Name() string {
	return constants.ProcessorSyncedDocument.String()
}

func (s *TextDocumentProcessor) Close() error {
	if s.pdfiumPool != nil {
		return s.pdfiumPool.Close()
	}
	return nil
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
	// Get a PDFium instance from the pool
	instance, err := s.pdfiumPool.GetInstance(30 * time.Second)
	if err != nil {
		return "", fmt.Errorf("failed to get PDFium instance: %w", err)
	}

	// Read the PDF file
	pdfBytes, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read PDF file %s: %w", filePath, err)
	}

	// Open the PDF document
	doc, err := instance.OpenDocument(&requests.OpenDocument{
		File: &pdfBytes,
	})
	if err != nil {
		return "", fmt.Errorf("failed to open PDF document %s: %w", filePath, err)
	}

	// Always close the document to release resources
	defer func() {
		if _, err := instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
			Document: doc.Document,
		}); err != nil {
			s.logger.Warn("failed to close PDF document", "filePath", filePath, "error", err)
		}
	}()

	// Get the page count
	pageCountResp, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get page count for PDF %s: %w", filePath, err)
	}

	var textBuilder strings.Builder

	// Extract text from each page
	for pageIndex := 0; pageIndex < pageCountResp.PageCount; pageIndex++ {
		// Get plain text for the page
		textResp, err := instance.GetPageText(&requests.GetPageText{
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{
					Document: doc.Document,
					Index:    pageIndex,
				},
			},
		})
		if err != nil {
			s.logger.Warn("failed to extract text from page",
				"page", pageIndex, "filePath", filePath, "error", err)
			continue
		}

		textBuilder.WriteString(textResp.Text)
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

func (s *TextDocumentProcessor) ProcessFile(ctx context.Context, filePath string) ([]memory.FileDocument, error) {
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

func (s *TextDocumentProcessor) processDirectory(ctx context.Context, dirPath string) ([]memory.FileDocument, error) {
	var allDocuments []memory.FileDocument

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

func (s *TextDocumentProcessor) processSingleFile(ctx context.Context, filePath string) ([]memory.FileDocument, error) {
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
		emptyDoc := memory.FileDocument{
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
		return []memory.FileDocument{emptyDoc}, nil
	}

	var documents []memory.FileDocument
	for i := 0; i < len(textContent); i += s.chunkSize {
		end := i + s.chunkSize
		if end > len(textContent) {
			end = len(textContent)
		}

		chunk := textContent[i:end]
		chunkIndex := i / s.chunkSize

		doc := memory.FileDocument{
			FieldID:        fmt.Sprintf("misc-%s-%d-chunk%d", fileName, timestamp.Unix(), chunkIndex),
			FieldContent:   chunk,
			FieldTimestamp: &timestamp,
			FieldSource:    s.Name(),
			FieldTags:      []string{"synced-document", "file", "document", ext},
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
