package google

import (
	"context"
	"fmt"
	"io"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const (
	SEARCH_FILES_TOOL_NAME = "search_drive_files"
	READ_FILE_TOOL_NAME    = "read_drive_file"
)

type SearchFilesArguments struct {
	Query     string `json:"query" jsonschema:"required,description=The query string to search for file titles"`
	PageToken string `json:"page_token,omitempty" jsonschema:"description=Optional page token for pagination."`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=Maximum number of files to return, default is 10."`
}

type ReadFileArguments struct {
	FileID string `json:"file_id" jsonschema:"required,description=The ID of the file to read."`
}

func processSearchFiles(ctx context.Context, accessToken string, args SearchFilesArguments) ([]*mcp_golang.Content, error) {
	driveService, err := getDriveService(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("error initializing Drive service: %w", err)
	}

	q := args.Query
	if q == "" {
		q = "trashed=false"
	} else {
		q = fmt.Sprintf("name contains '%s'", q)
	}

	limit := args.Limit
	if limit <= 0 {
		limit = 10 // Default limit
	}
	request := driveService.Files.List().
		Q(q).
		PageSize(int64(limit)).
		Fields("nextPageToken, files(id, name, mimeType, modifiedTime)").
		SupportsAllDrives(true).         // Crucial for accessing shared drives
		IncludeItemsFromAllDrives(true). // Ensure files from shared drives are included
		Corpora("allDrives")

	if args.PageToken != "" {
		request = request.PageToken(args.PageToken)
	}

	fileList, err := request.Do()
	if err != nil {
		return nil, fmt.Errorf("error searching files: %v", err)
	}

	contents := []*mcp_golang.Content{}

	for _, file := range fileList.Files {
		contents = append(contents, &mcp_golang.Content{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: fmt.Sprintf("File: %s - %s, Modified: %s, Type: %s", file.Name, file.Id, file.ModifiedTime, file.MimeType),
			},
		})
	}

	return contents, nil
}

func processReadFile(ctx context.Context, accessToken string, args ReadFileArguments) ([]*mcp_golang.Content, error) {
	driveService, err := getDriveService(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("error initializing Drive service: %w", err)
	}

	if args.FileID == "" {
		return nil, fmt.Errorf("file ID cannot be empty")
	}

	// Get file metadata first to check the MIME type and get useful info
	fileMeta, err := driveService.Files.Get(args.FileID).Fields("id", "name", "mimeType", "webViewLink", "size").SupportsAllDrives(true).Do()
	if err != nil {
		fmt.Println("Error retrieving file metadata", err)
		return []*mcp_golang.Content{
			{
				Type: "text",
				TextContent: &mcp_golang.TextContent{
					Text: fmt.Sprintf("Error retrieving file metadata for ID %s: %v", args.FileID, err),
				},
			},
		}, nil
	}

	var contentText string

	// Handle Google Docs, Sheets, Slides by exporting them
	switch fileMeta.MimeType {
	case "application/vnd.google-apps.document":
		resp, err := driveService.Files.Export(args.FileID, "text/plain").Download()
		if err != nil {
			return nil, fmt.Errorf("unable to export Google Doc '%s' (ID: %s): %w", fileMeta.OriginalFilename, args.FileID, err)
		}
		defer resp.Body.Close() //nolint:errcheck
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("unable to read exported Google Doc content for '%s' (ID: %s): %w", fileMeta.OriginalFilename, args.FileID, err)
		}
		contentText = fmt.Sprintf("Content of Google Doc '%s':\n%s", fileMeta.OriginalFilename, string(bodyBytes))

	case "application/vnd.google-apps.spreadsheet":
		// Exporting as CSV
		resp, err := driveService.Files.Export(args.FileID, "text/csv").Download()
		if err != nil {
			return nil, fmt.Errorf("unable to export Google Sheet '%s' (ID: %s) as CSV: %w", fileMeta.OriginalFilename, args.FileID, err)
		}
		defer resp.Body.Close() //nolint:errcheck
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("unable to read exported Google Sheet content for '%s' (ID: %s): %w", fileMeta.OriginalFilename, args.FileID, err)
		}
		contentText = fmt.Sprintf("Content of Google Sheet '%s' (CSV format):\n%s", fileMeta.OriginalFilename, string(bodyBytes))

	case "application/vnd.google-apps.presentation":
		// Exporting as plain text
		resp, err := driveService.Files.Export(args.FileID, "text/plain").Download()
		if err != nil {
			return nil, fmt.Errorf("unable to export Google Slides '%s' (ID: %s) as text: %w", fileMeta.OriginalFilename, args.FileID, err)
		}
		defer resp.Body.Close() //nolint:errcheck
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("unable to read exported Google Slides content for '%s' (ID: %s): %w", fileMeta.OriginalFilename, args.FileID, err)
		}
		contentText = fmt.Sprintf("Content of Google Slides '%s' (Text format):\n%s", fileMeta.OriginalFilename, string(bodyBytes))

	default:
		// For other file types, try direct download
		// Check size before attempting download to avoid large files
		const maxDownloadSize = 5 * 1024 * 1024 // 5 MB limit
		if fileMeta.Size > maxDownloadSize {
			return []*mcp_golang.Content{
				{
					Type: "text",
					TextContent: &mcp_golang.TextContent{
						Text: fmt.Sprintf("File '%s' (ID: %s, Type: %s) is too large (%d bytes) to download directly. Maximum size is %d bytes. Use the webViewLink: %s", fileMeta.OriginalFilename, fileMeta.Id, fileMeta.MimeType, fileMeta.Size, maxDownloadSize, fileMeta.WebViewLink),
					},
				},
			}, nil
		}

		resp, err := driveService.Files.Get(args.FileID).SupportsAllDrives(true).Download()
		if err != nil {
			if gerr, ok := err.(*googleapi.Error); ok && gerr.Code == 403 {
				// Handle specific errors like inability to download Google Apps Script etc.
				return []*mcp_golang.Content{
					{
						Type: "text",
						TextContent: &mcp_golang.TextContent{
							Text: fmt.Sprintf("Could not directly download file '%s' (ID: %s, Type: %s). This might be due to file type restrictions (e.g., Google Apps Script) or permissions. Try the web view link: %s", fileMeta.OriginalFilename, fileMeta.Id, fileMeta.MimeType, fileMeta.WebViewLink),
						},
					},
				}, nil
			}
			return nil, fmt.Errorf("unable to download file content for '%s' (ID: %s): %w", fileMeta.OriginalFilename, args.FileID, err)
		}
		defer resp.Body.Close() //nolint:errcheck

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("unable to read downloaded file content for '%s' (ID: %s): %w", fileMeta.OriginalFilename, args.FileID, err)
		}

		// Basic check for binary data
		isBinary := false
		for _, b := range bodyBytes {
			if b == 0 {
				isBinary = true
				break
			}
		}

		if isBinary {
			contentText = fmt.Sprintf("File '%s' (ID: %s, Type: %s) appears to be binary and cannot be displayed as text. Web view link: %s", fileMeta.OriginalFilename, fileMeta.Id, fileMeta.MimeType, fileMeta.WebViewLink)
		} else {
			// Limit text output size
			const maxTextBytes = 10000 // 10KB limit for text display
			if len(bodyBytes) > maxTextBytes {
				contentText = fmt.Sprintf("Content of '%s' (first %d bytes):\n%s...", fileMeta.OriginalFilename, maxTextBytes, string(bodyBytes[:maxTextBytes]))
			} else {
				contentText = fmt.Sprintf("Content of '%s':\n%s", fileMeta.OriginalFilename, string(bodyBytes))
			}
		}
	}

	// Limit overall contentText size again just in case
	const maxReturnLength = 15000
	if len(contentText) > maxReturnLength {
		contentText = contentText[:maxReturnLength] + "... (truncated)"
	}

	return []*mcp_golang.Content{
		{
			Type: "text",
			TextContent: &mcp_golang.TextContent{
				Text: contentText,
			},
		},
	}, nil
}

func getDriveService(ctx context.Context, accessToken string) (*drive.Service, error) {
	token := &oauth2.Token{
		AccessToken: accessToken,
	}
	config := oauth2.Config{}
	client := config.Client(ctx, token)

	driveService, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("error initializing Drive service: %w", err)
	}
	return driveService, nil
}
