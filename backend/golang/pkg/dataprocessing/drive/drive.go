package drive

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/processor"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type DriveProcessor struct {
	store  *db.Store
	logger *log.Logger
}

func NewDriveProcessor(store *db.Store, logger *log.Logger) (processor.Processor, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is nil")
	}

	if store == nil {
		return nil, fmt.Errorf("store is nil")
	}

	return &DriveProcessor{store: store, logger: logger}, nil
}

func (g *DriveProcessor) Name() string { return "drive" }

func (g *DriveProcessor) ProcessFile(ctx context.Context, path string) ([]types.Record, error) {
	return nil, fmt.Errorf("ProcessFile not supported for Google Drive - use Sync instead")
}

func (g *DriveProcessor) ProcessDirectory(ctx context.Context, dir string) ([]types.Record, error) {
	return nil, fmt.Errorf("ProcessDirectory not supported for Google Drive - use Sync instead")
}

func (g *DriveProcessor) Sync(ctx context.Context, token string) ([]types.Record, bool, error) {
	g.logger.Info("Syncing Google Drive")
	records, hasMore, _, err := g.SyncWithPagination(ctx, token, "", 100)
	return records, hasMore, err
}

func (g *DriveProcessor) SyncWithPagination(ctx context.Context, token, pageToken string, maxResults int) ([]types.Record, bool, string, error) {
	g.logger.Info("Syncing Google Drive with pagination")
	srv, err := getDriveService(ctx, token)
	if err != nil {
		return nil, false, "", fmt.Errorf("failed to get drive service: %w", err)
	}

	// Query for image files
	query := "mimeType contains 'image/' and trashed=false"

	call := srv.Files.List().Q(query).Fields("nextPageToken,files(id,name,mimeType,createdTime,modifiedTime,webViewLink,thumbnailLink,size)")

	// Set page size
	if maxResults > 0 {
		call = call.PageSize(int64(maxResults))
	}

	// Set page token for pagination
	if pageToken != "" {
		call = call.PageToken(pageToken)
	}

	files, err := call.Do()
	if err != nil {
		return nil, false, "", fmt.Errorf("failed to list files: %w", err)
	}

	var records []types.Record
	for _, file := range files.Files {
		g.logger.Info("File", "file", file)
		record, err := g.createImageRecord(file)
		if err != nil {
			g.logger.Warn("Failed to create record for file", "fileId", file.Id, "fileName", file.Name, "error", err)
			continue
		}
		records = append(records, record)
	}

	hasMore := files.NextPageToken != ""
	g.logger.Info("Fetched images from Google Drive", "count", len(records), "hasMore", hasMore)

	// Return the records, hasMore flag, and nextPageToken for the interface that needs it
	return records, hasMore, files.NextPageToken, nil
}

// SyncAll fetches all images across all pages.
func (g *DriveProcessor) SyncAll(ctx context.Context, token string) ([]types.Record, error) {
	g.logger.Info("Syncing all images from Google Drive")
	var allRecords []types.Record
	pageToken := ""

	for {
		records, hasMore, nextPageToken, err := g.SyncWithPagination(ctx, token, pageToken, 100)
		if err != nil {
			return nil, err
		}

		allRecords = append(allRecords, records...)

		if !hasMore || nextPageToken == "" {
			break
		}

		pageToken = nextPageToken
	}

	g.logger.Info("Fetched all images from Google Drive", "totalCount", len(allRecords))
	return allRecords, nil
}

func (g *DriveProcessor) createImageRecord(file *drive.File) (types.Record, error) {
	var createdTime time.Time
	if file.CreatedTime != "" {
		if t, err := time.Parse(time.RFC3339, file.CreatedTime); err == nil {
			createdTime = t
		}
	}

	data := map[string]interface{}{
		"id":            file.Id,
		"name":          file.Name,
		"mimeType":      file.MimeType,
		"webViewLink":   file.WebViewLink,
		"thumbnailLink": file.ThumbnailLink,
		"createdTime":   file.CreatedTime,
		"modifiedTime":  file.ModifiedTime,
		"size":          file.Size,
	}

	return types.Record{
		Data:      data,
		Timestamp: createdTime,
		Source:    g.Name(),
	}, nil
}

func (g *DriveProcessor) ToDocuments(ctx context.Context, recs []types.Record) ([]memory.Document, error) {
	var out []memory.ConversationDocument

	for _, r := range recs {
		get := func(k string) string {
			if v, ok := r.Data[k]; ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
			return ""
		}

		fileName := get("name")
		if fileName == "" {
			continue
		}

		user := ""
		sourceUsername, err := g.store.GetSourceUsername(ctx, "drive")
		if err != nil {
			g.logger.Error("get source username", "error", err)
		}
		if sourceUsername != nil {
			user = sourceUsername.Username
		}

		// Create a unique document ID based on file ID
		fileID := get("id")
		hasher := sha256.New()
		hasher.Write([]byte(fileID))
		imageHash := fmt.Sprintf("%x", hasher.Sum(nil))[:16]
		documentID := fmt.Sprintf("drive-image-%s", imageHash)

		out = append(out, memory.ConversationDocument{
			FieldID:     documentID,
			User:        user,
			People:      []string{user},
			FieldSource: "drive",
			FieldTags:   []string{"image", "file"},
			FieldMetadata: map[string]string{
				"id":            fileID,
				"name":          fileName,
				"mimeType":      get("mimeType"),
				"webViewLink":   get("webViewLink"),
				"thumbnailLink": get("thumbnailLink"),
				"createdTime":   get("createdTime"),
				"modifiedTime":  get("modifiedTime"),
			},
			Conversation: []memory.ConversationMessage{
				{
					Content: fmt.Sprintf("Image file: %s", fileName),
					Speaker: user,
					Time:    r.Timestamp,
				},
			},
		})
	}

	var documents []memory.Document
	for _, document := range out {
		documents = append(documents, &document)
	}

	return documents, nil
}

var getDriveService = func(ctx context.Context, accessToken string) (*drive.Service, error) {
	token := &oauth2.Token{
		AccessToken: accessToken,
	}
	config := oauth2.Config{}
	client := config.Client(ctx, token)

	return drive.NewService(ctx, option.WithHTTPClient(client))
}
