package longmemeval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/constants"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// LongMemEvalRecord represents a single record in the LongMemEval dataset
type LongMemEvalRecord struct {
	QuestionID         string          `json:"question_id"`
	QuestionType       string          `json:"question_type"`
	Question           string          `json:"question"`
	Answer             interface{}     `json:"answer"` // Can be string or number
	QuestionDate       string          `json:"question_date"`
	HaystackSessionIDs []string        `json:"haystack_session_ids"`
	HaystackDates      []string        `json:"haystack_dates"`
	HaystackSessions   [][]SessionTurn `json:"haystack_sessions"`
	AnswerSessionIDs   []string        `json:"answer_session_ids"`
}

// SessionTurn represents a single turn in a conversation session
type SessionTurn struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	HasAnswer *bool  `json:"has_answer,omitempty"`
}

type LongMemEvalProcessor struct {
	store  *db.Store
	logger *log.Logger
}

func NewLongMemEvalProcessor(store *db.Store, logger *log.Logger) (*LongMemEvalProcessor, error) {
	if store == nil {
		return nil, fmt.Errorf("store is nil")
	}

	if logger == nil {
		return nil, fmt.Errorf("logger is nil")
	}
	return &LongMemEvalProcessor{store: store, logger: logger}, nil
}

// convertAnswerToString converts an answer field (which can be string or number) to string
func convertAnswerToString(answer interface{}) string {
	switch v := answer.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v) // Remove decimal for whole numbers
	case int:
		return strconv.Itoa(v)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v) // Fallback for any other type
	}
}

func (s *LongMemEvalProcessor) Name() string {
	return "longmemeval"
}

func parseQuestionDate(dateStr string) (time.Time, error) {
	// Try parsing different date formats that might be used
	formats := []string{
		"2006/01/02 (Mon) 15:04",  // LongMemEval format: "2023/05/30 (Tue) 23:59"
		"2006-01-02",
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006/01/02",
	}
	
	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}
	
	// If all parsing fails, return current time with a warning
	return time.Now(), fmt.Errorf("could not parse date: %s", dateStr)
}

// ProcessFile implements the DocumentProcessor interface - direct ConversationDocument output.
func (s *LongMemEvalProcessor) ProcessFile(ctx context.Context, filePath string) ([]memory.ConversationDocument, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var records []LongMemEvalRecord
	if err := json.Unmarshal(jsonData, &records); err != nil {
		return nil, err
	}

	var documents []memory.ConversationDocument

	for i, record := range records {
		if record.QuestionID == "" {
			s.logger.Debug("Skipping record with empty question_id", "index", i)
			continue
		}

		// Parse question date
		questionTime, err := parseQuestionDate(record.QuestionDate)
		if err != nil {
			s.logger.Warn("Failed to parse question date", "index", i, "date", record.QuestionDate, "error", err)
			questionTime = time.Now() // fallback to current time
		}

		// Process each haystack session as a separate conversation document
		for sessionIdx, session := range record.HaystackSessions {
			if len(session) == 0 {
				continue
			}

			var messages []memory.ConversationMessage
			var participants []string
			participantSet := make(map[string]bool)

			// Convert session turns to conversation messages
			for _, turn := range session {
				if turn.Role == "" || turn.Content == "" {
					continue
				}

				// Track participants
				if !participantSet[turn.Role] {
					participantSet[turn.Role] = true
					participants = append(participants, turn.Role)
				}

				messages = append(messages, memory.ConversationMessage{
					Speaker: turn.Role,
					Content: turn.Content,
					Time:    questionTime, // Use question time as base timestamp
				})
			}

			if len(messages) == 0 {
				continue
			}

			// Create unique ID for this session
			sessionID := fmt.Sprintf("%s_session_%d", record.QuestionID, sessionIdx)

			// Determine primary user (assume "user" role if present, otherwise first participant)
			primaryUser := "user"
			if !participantSet["user"] && len(participants) > 0 {
				primaryUser = participants[0]
			}

			// Create metadata with LongMemEval-specific information
			metadata := map[string]string{
				"question_id":      record.QuestionID,
				"question_type":    record.QuestionType,
				"question":         record.Question,
				"expected_answer":  convertAnswerToString(record.Answer),
				"session_index":    strconv.Itoa(sessionIdx),
				"question_date":    record.QuestionDate,
			}

			// Add session ID if available
			if sessionIdx < len(record.HaystackSessionIDs) {
				metadata["haystack_session_id"] = record.HaystackSessionIDs[sessionIdx]
			}

			// Add session date if available
			if sessionIdx < len(record.HaystackDates) {
				metadata["haystack_date"] = record.HaystackDates[sessionIdx]
			}

			// Mark if this session contains evidence
			isEvidenceSession := false
			for _, evidenceSessionID := range record.AnswerSessionIDs {
				if sessionIdx < len(record.HaystackSessionIDs) && 
				   record.HaystackSessionIDs[sessionIdx] == evidenceSessionID {
					isEvidenceSession = true
					break
				}
			}
			if isEvidenceSession {
				metadata["contains_evidence"] = "true"
			}

			// Check if any turns have the has_answer flag
			hasAnswerTurns := false
			for _, turn := range session {
				if turn.HasAnswer != nil && *turn.HasAnswer {
					hasAnswerTurns = true
					break
				}
			}
			if hasAnswerTurns {
				metadata["has_answer_turns"] = "true"
			}

			conversationDoc := memory.ConversationDocument{
				FieldID:       sessionID,
				FieldSource:   constants.ProcessorLongMemEval.String(),
				FieldTags:     []string{"longmemeval", "evaluation", "qa", record.QuestionType},
				FieldMetadata: metadata,
				Conversation:  messages,
				User:          primaryUser,
				People:        participants,
			}

			documents = append(documents, conversationDoc)
		}
	}

	s.logger.Info("LongMemEval processing completed", 
		"records_processed", len(records), 
		"documents_created", len(documents))

	return documents, nil
}