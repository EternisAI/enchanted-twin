package whatsapp

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	_ "github.com/mattn/go-sqlite3"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/processor"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type WhatsappProcessor struct {
	store  *db.Store
	logger *log.Logger
}

func NewWhatsappProcessor(store *db.Store, logger *log.Logger) processor.Processor {
	return &WhatsappProcessor{store: store, logger: logger}
}

func (s *WhatsappProcessor) Name() string {
	return "whatsapp"
}

func (s *WhatsappProcessor) ReadWhatsAppDB(ctx context.Context, dbPath string) ([]types.Record, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			s.logger.Warn("Error closing database", "error", err)
		}
	}()

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled before query: %w", err)
	}

	query := `SELECT 
		m.Z_PK, m.ZISFROMME, m.ZCHATSESSION, m.ZMESSAGEINFO, m.ZMESSAGEDATE, m.ZSENTDATE,
		m.ZFROMJID, m.ZTEXT, m.ZTOJID,
		CASE 
			WHEN m.ZISFROMME = 1 THEN s.ZPARTNERNAME 
			ELSE '' 
		END AS ZTONAME,
		CASE 
			WHEN m.ZISFROMME = 0 THEN s.ZPARTNERNAME 
			ELSE '' 
		END AS ZFROMNAME
		FROM ZWAMESSAGE m
		LEFT JOIN ZWACHATSESSION s ON m.ZCHATSESSION = s.Z_PK
		WHERE m.ZTEXT IS NOT NULL`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			s.logger.Warn("Error closing rows", "error", err)
		}
	}()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get column names: %v", err)
	}

	count := len(columns)
	values := make([]interface{}, count)
	valuePtrs := make([]interface{}, count)
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	var records []types.Record
	rowCount := 0
	for rows.Next() {
		if rowCount%100 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("context canceled during row processing: %w", err)
			}
		}

		err := rows.Scan(valuePtrs...)
		if err != nil {
			s.logger.Warn("Scan error", "error", err, "row", rowCount, "expected", count)
			return nil, fmt.Errorf("scan failed: %v", err)
		}
		rowCount++

		data := make(map[string]interface{})
		var timestamp time.Time

		importantFields := map[string]bool{
			"ZTEXT":     true,
			"ZISFROMME": true,
			"ZFROMJID":  true,
			"ZTOJID":    true,
			"ZFROMNAME": true,
			"ZTONAME":   true,
		}

		foundFields := make(map[string]bool)

		for i, col := range columns {
			val := values[i]

			if importantFields[col] {
				foundFields[col] = true
			}

			simplifiedKey := col
			if len(col) > 1 && col[0] == 'Z' {
				simplifiedKey = col[1:]
			}
			simplifiedKey = strings.ToLower(simplifiedKey)

			switch col {
			case "ZMESSAGEDATE", "ZSENTDATE":

				if v, ok := val.(int64); ok {
					t := time.Unix(v, 0)
					data[simplifiedKey] = t
					if col == "ZMESSAGEDATE" {
						timestamp = t
					}
				} else if v, ok := val.(float64); ok {
					t := time.Unix(int64(v), 0)
					data[simplifiedKey] = t
					if col == "ZMESSAGEDATE" {
						timestamp = t
					}
				} else {
					data[simplifiedKey] = val
				}
			default:
				data[simplifiedKey] = val
			}
		}

		for field := range importantFields {
			if !foundFields[field] {
				s.logger.Warn("Important field not found in query results", "field", field)
			}
		}

		if timestamp.IsZero() {
			timestamp = time.Now()
		}

		record := types.Record{
			Data:      data,
			Timestamp: timestamp,
			Source:    "whatsapp",
		}

		records = append(records, record)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during iteration: %v", err)
	}

	return records, nil
}

func (s *WhatsappProcessor) ProcessDirectory(ctx context.Context, filePath string) ([]types.Record, error) {
	return nil, fmt.Errorf("sync operation not supported for WhatsApp")
}

func (s *WhatsappProcessor) ProcessFile(ctx context.Context, filePath string) ([]types.Record, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store is nil")
	}

	return s.ReadWhatsAppDB(ctx, filePath)
}

func (s *WhatsappProcessor) Sync(ctx context.Context, accessToken string) ([]types.Record, bool, error) {
	return nil, false, fmt.Errorf("sync operation not supported for WhatsApp")
}

func (s *WhatsappProcessor) ToDocuments(ctx context.Context, records []types.Record) ([]memory.Document, error) {
	if len(records) == 0 {
		return []memory.Document{}, nil
	}
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	conversationMap := make(map[string]*memory.ConversationDocument)

	for _, record := range records {
		content, ok := record.Data["text"].(string)
		if !ok || strings.TrimSpace(content) == "" {
			continue
		}

		chatSessionInterface, ok := record.Data["chatsession"]
		if !ok {
			continue
		}

		var chatSession string
		switch v := chatSessionInterface.(type) {
		case int:
			chatSession = fmt.Sprintf("%d", v)
		case int64:
			chatSession = fmt.Sprintf("%d", v)
		case float64:
			chatSession = fmt.Sprintf("%.0f", v)
		case string:
			chatSession = v
		default:
			continue
		}

		isFromMe, ok := record.Data["isfromme"]
		if !ok {
			continue
		}

		var fromMe bool
		switch v := isFromMe.(type) {
		case int:
			fromMe = v == 1
		case int64:
			fromMe = v == 1
		case float64:
			fromMe = v == 1
		case bool:
			fromMe = v
		default:
			continue
		}

		var speaker string
		var participants []string

		if fromMe {
			speaker = "me"
			toName, _ := record.Data["toname"].(string)
			if strings.TrimSpace(toName) != "" {
				participants = []string{"me", toName}
			} else {
				participants = []string{"me"}
			}
		} else {
			fromName, _ := record.Data["fromname"].(string)
			if strings.TrimSpace(fromName) != "" {
				speaker = fromName
				participants = []string{"me", fromName}
			} else {
				speaker = "unknown"
				participants = []string{"me", "unknown"}
			}
		}

		conversation, exists := conversationMap[chatSession]
		if !exists {
			conversationMap[chatSession] = &memory.ConversationDocument{
				FieldID:      fmt.Sprintf("whatsapp-chat-%s", chatSession),
				FieldSource:  "whatsapp",
				FieldTags:    []string{"whatsapp", "conversation", "chat"},
				People:       participants,
				User:         "me",
				Conversation: []memory.ConversationMessage{},
				FieldMetadata: map[string]string{
					"type":         "conversation",
					"chat_session": chatSession,
				},
			}
			conversation = conversationMap[chatSession]
		}

		conversation.Conversation = append(conversation.Conversation, memory.ConversationMessage{
			Speaker: speaker,
			Content: content,
			Time:    record.Timestamp,
		})

		peopleMap := make(map[string]bool)
		for _, person := range conversation.People {
			peopleMap[person] = true
		}
		for _, participant := range participants {
			if !peopleMap[participant] {
				conversation.People = append(conversation.People, participant)
				peopleMap[participant] = true
			}
		}
	}

	var conversationDocuments []memory.ConversationDocument
	for _, conversation := range conversationMap {
		if len(conversation.Conversation) > 0 {
			conversationDocuments = append(conversationDocuments, *conversation)
		}
	}

	return memory.ConversationDocumentsToDocuments(conversationDocuments), nil
}
