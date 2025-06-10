package whatsapp

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"regexp"
	"sort"
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// convertCoreDataTimestamp converts Core Data timestamp to Unix timestamp
// Core Data timestamps are seconds since 2001-01-01 00:00:00 UTC
func convertCoreDataTimestamp(coreDataTimestamp float64) time.Time {
	// Validate the timestamp - Core Data timestamps should be positive and reasonable
	// Negative values or extremely large values are likely invalid
	if coreDataTimestamp < 0 || coreDataTimestamp > 2147483647 { // Max int32 for reasonable future dates
		return time.Time{} // Return zero time for invalid timestamps
	}

	// Core Data reference date: January 1, 2001, 00:00:00 UTC
	referenceDate := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	result := referenceDate.Add(time.Duration(coreDataTimestamp) * time.Second)

	// Additional validation - ensure the result is within a reasonable range
	// WhatsApp was founded in 2009, so messages before 2009 are likely invalid
	minDate := time.Date(2009, 1, 1, 0, 0, 0, 0, time.UTC)
	maxDate := time.Now().Add(24 * time.Hour) // Allow up to 1 day in the future

	if result.Before(minDate) || result.After(maxDate) {
		return time.Time{} // Return zero time for unreasonable dates
	}

	return result
}

// convertWhatsAppTimestamp tries multiple timestamp conversion strategies
func convertWhatsAppTimestamp(rawTimestamp interface{}) time.Time {
	var timestampFloat float64

	switch v := rawTimestamp.(type) {
	case int64:
		timestampFloat = float64(v)
	case float64:
		timestampFloat = v
	case int:
		timestampFloat = float64(v)
	default:
		return time.Time{} // Invalid type
	}

	// If the timestamp is 0 or negative, it's invalid
	if timestampFloat <= 0 {
		return time.Time{}
	}

	// More permissive date range - allow any reasonable date
	minDate := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	maxDate := time.Now().Add(365 * 24 * time.Hour) // Allow up to 1 year in the future

	// Strategy 1: Try Unix timestamp in seconds (most common)
	if timestampFloat > 946684800 && timestampFloat < 4102444800 { // Between 2000 and 2100
		unixResult := time.Unix(int64(timestampFloat), 0)
		if unixResult.After(minDate) && unixResult.Before(maxDate) {
			return unixResult
		}
	}

	// Strategy 2: Try Unix timestamp in milliseconds
	if timestampFloat > 946684800000 && timestampFloat < 4102444800000 { // Between 2000 and 2100 in milliseconds
		unixMillisResult := time.Unix(int64(timestampFloat/1000), int64(timestampFloat)%1000*1000000)
		if unixMillisResult.After(minDate) && unixMillisResult.Before(maxDate) {
			return unixMillisResult
		}
	}

	// Strategy 3: Try Core Data timestamp (seconds since 2001-01-01) - more permissive
	if timestampFloat > 0 && timestampFloat < 3155760000 { // Up to year 2100
		referenceDate := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
		coreDataResult := referenceDate.Add(time.Duration(timestampFloat) * time.Second)
		if coreDataResult.After(minDate) && coreDataResult.Before(maxDate) {
			return coreDataResult
		}
	}

	// Strategy 4: Try Mac Cocoa timestamp - even more permissive
	if timestampFloat > -978307200 && timestampFloat < 3155760000 { // Allow negative values for dates before 2001
		cocoaResult := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(timestampFloat) * time.Second)
		// Only check if it's not too far in the past or future
		veryMinDate := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
		if cocoaResult.After(veryMinDate) && cocoaResult.Before(maxDate) {
			return cocoaResult
		}
	}

	// Strategy 5: If all else fails, try to make something reasonable from the timestamp
	// This is a last resort to avoid losing messages
	if timestampFloat > 0 {
		// Try different epoch interpretations
		epochs := []time.Time{
			time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC), // Mac/Core Data epoch
			time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), // Unix epoch
			time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC), // Some systems use 1900
		}

		for _, epoch := range epochs {
			result := epoch.Add(time.Duration(timestampFloat) * time.Second)
			if result.After(minDate) && result.Before(maxDate) {
				return result
			}
		}
	}

	// If we get here, log the problematic timestamp for debugging
	// Note: We can't use logger here since this is a standalone function
	// The calling code will log the failure

	return time.Time{} // All strategies failed
}

// validateAndFixTimestamp validates and potentially corrects time.Time values from the database
func validateAndFixTimestamp(t time.Time) time.Time {
	// If the timestamp is zero, return it as-is
	if t.IsZero() {
		return t
	}

	// Define reasonable date ranges for WhatsApp messages
	minDate := time.Date(2009, 1, 1, 0, 0, 0, 0, time.UTC) // WhatsApp founded in 2009
	maxDate := time.Now().Add(24 * time.Hour)              // Allow up to 1 day in the future

	// If the timestamp is within reasonable range, use it as-is
	if t.After(minDate) && t.Before(maxDate) {
		return t
	}

	// If the timestamp is way too old (like 1993), it's likely a database artifact
	// These seem to be Core Data timestamps that weren't converted properly
	if t.Year() < 2000 {
		// Try to interpret the timestamp as seconds since the Core Data epoch
		// Core Data epoch is 2001-01-01 00:00:00 UTC
		coreDataEpoch := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

		// Calculate seconds since Unix epoch for the given timestamp
		secondsSinceUnixEpoch := t.Unix()

		// Try interpreting as seconds since Core Data epoch
		correctedTime := coreDataEpoch.Add(time.Duration(secondsSinceUnixEpoch) * time.Second)
		if correctedTime.After(minDate) && correctedTime.Before(maxDate) {
			return correctedTime
		}

		// Try interpreting the timestamp components differently
		// Sometimes the year/month/day are stored incorrectly but time components are valid
		now := time.Now()

		// Try using today's date with the time component
		todayWithTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
		if todayWithTime.Before(maxDate) {
			// Use a date from a few years ago to make it historical
			historicalDate := todayWithTime.AddDate(-2, 0, 0)
			return historicalDate
		}
	}

	// If the timestamp is too far in the future, cap it to now
	if t.After(maxDate) {
		return time.Now()
	}

	// If all else fails, return zero time to trigger fallback logic
	return time.Time{}
}

// isWhatsAppJID checks if a string looks like a WhatsApp JID
func isWhatsAppJID(text string) bool {
	// Check for patterns like "16672940017@s.whatsapp.net" or just phone numbers followed by @
	whatsappJIDPattern := regexp.MustCompile(`^\d{10,15}@s\.whatsapp\.net$`)
	phoneAtPattern := regexp.MustCompile(`^\d{10,15}@`)

	return whatsappJIDPattern.MatchString(text) || phoneAtPattern.MatchString(text)
}

// shouldFilterMessage determines if a message should be filtered out
func shouldFilterMessage(text, groupName string) bool {
	text = strings.TrimSpace(text)

	// Filter empty messages
	if text == "" {
		return true
	}

	// Filter WhatsApp JIDs
	if isWhatsAppJID(text) {
		return true
	}

	// Filter group names appearing as content
	if groupName != "" && text == groupName {
		return true
	}

	// Filter messages that are just phone numbers
	phonePattern := regexp.MustCompile(`^\d{10,15}$`)
	if phonePattern.MatchString(text) {
		return true
	}

	return false
}

// resolveTagsInMessage resolves @mentions in message content
func resolveTagsInMessage(content string, contactNames map[string]string, participantsMap map[string]bool) string {
	// Pattern to match @mentions like @15083108164
	mentionPattern := regexp.MustCompile(`@(\d{10,15})`)

	return mentionPattern.ReplaceAllStringFunc(content, func(match string) string {
		// Extract the phone number from the mention
		phoneNumber := strings.TrimPrefix(match, "@")

		// Try to resolve to contact name
		if contactName, exists := contactNames[phoneNumber]; exists && contactName != "" && !strings.Contains(contactName, "=") {
			return "@" + contactName
		}

		// Try with full JID format
		fullJID := phoneNumber + "@s.whatsapp.net"
		if contactName, exists := contactNames[fullJID]; exists && contactName != "" && !strings.Contains(contactName, "=") {
			return "@" + contactName
		}

		// Try to find in participants map by checking if any participant contains this phone number
		for participant := range participantsMap {
			// Skip "me" and other non-phone participants
			if participant == "me" || participant == "unknown" {
				continue
			}

			// Check if participant name contains the phone number or vice versa
			if strings.Contains(participant, phoneNumber) || strings.Contains(phoneNumber, participant) {
				return "@" + participant
			}
		}

		// Try reverse lookup - check if any contact name maps to this phone number
		for jid, name := range contactNames {
			if strings.Contains(jid, phoneNumber) && name != "" && !strings.Contains(name, "=") {
				return "@" + name
			}
		}

		// Return original if no resolution found
		return match
	})
}

// decodeJID attempts to decode a JID that might be base64 encoded or compressed
func decodeJID(jid string) string {
	if jid == "" {
		return ""
	}

	// If it looks like base64 (contains = or has suspicious length)
	if strings.Contains(jid, "=") || (len(jid) < 10 && len(jid) > 0) {
		// Special case for very short base64 strings like "IAA="
		if len(jid) <= 4 && strings.HasSuffix(jid, "=") {
			// This is likely corrupted data, return empty
			return ""
		}

		// Try base64 decoding
		decoded, err := base64.StdEncoding.DecodeString(jid)
		if err == nil && len(decoded) > 0 {
			// Check if decoded value looks like a valid JID
			decodedStr := string(decoded)
			// Filter out non-printable characters
			if isPrintableString(decodedStr) && (strings.Contains(decodedStr, "@") || (len(decodedStr) >= 10 && len(decodedStr) <= 50)) {
				return decodedStr
			}
		}

		// Try URL-safe base64 decoding
		decoded, err = base64.URLEncoding.DecodeString(jid)
		if err == nil && len(decoded) > 0 {
			decodedStr := string(decoded)
			if isPrintableString(decodedStr) && (strings.Contains(decodedStr, "@") || (len(decodedStr) >= 10 && len(decodedStr) <= 50)) {
				return decodedStr
			}
		}

		// Try raw base64 decoding (no padding)
		decoded, err = base64.RawStdEncoding.DecodeString(jid)
		if err == nil && len(decoded) > 0 {
			decodedStr := string(decoded)
			if isPrintableString(decodedStr) && (strings.Contains(decodedStr, "@") || (len(decodedStr) >= 10 && len(decodedStr) <= 50)) {
				return decodedStr
			}
		}

		// If it's a short base64-like string that couldn't be decoded properly, return empty
		if len(jid) < 10 {
			return ""
		}
	}

	// Return original if not encoded or decoding failed
	return jid
}

// isPrintableString checks if a string contains only printable characters
func isPrintableString(s string) bool {
	for _, r := range s {
		if r < 32 || r > 126 {
			return false
		}
	}
	return true
}

// extractSenderFromJID extracts the sender name from a WhatsApp JID
// For group messages, JID format is typically: phoneNumber@s.whatsapp.net
// For messages within groups, it might be: groupJID/senderPhoneNumber@s.whatsapp.net
func extractSenderFromJID(jid string) string {
	if jid == "" {
		return ""
	}

	// Clean up the JID - remove any whitespace
	jid = strings.TrimSpace(jid)

	// Handle group message format (groupJID/senderJID)
	if idx := strings.LastIndex(jid, "/"); idx > 0 {
		jid = jid[idx+1:]
	}

	// Extract phone number or username before @ symbol
	if idx := strings.Index(jid, "@"); idx > 0 {
		phoneNumber := jid[:idx]
		// Validate that we got a reasonable phone number
		if len(phoneNumber) >= 5 && !strings.Contains(phoneNumber, "=") {
			return phoneNumber
		}
	}

	// If the JID doesn't look valid, return empty string
	if strings.Contains(jid, "=") || len(jid) < 5 {
		return ""
	}

	return jid
}

// loadContactNames loads a mapping of JIDs to contact names from the WhatsApp database
func (s *WhatsappProcessor) loadContactNames(ctx context.Context, db *sql.DB) (map[string]string, error) {
	contactMap := make(map[string]string)

	// Try multiple queries to find contact information
	queries := []struct {
		name  string
		query string
	}{
		{
			name:  "ZWAPROFILEPUSHNAME",
			query: `SELECT ZJID, ZPUSHNAME FROM ZWAPROFILEPUSHNAME WHERE ZJID IS NOT NULL AND ZPUSHNAME IS NOT NULL AND ZPUSHNAME != '' AND ZPUSHNAME != 'IAA='`,
		},
		{
			name:  "ZWACONTACT",
			query: `SELECT ZJID, ZFULLNAME FROM ZWACONTACT WHERE ZJID IS NOT NULL AND ZFULLNAME IS NOT NULL AND ZFULLNAME != '' AND ZFULLNAME != 'IAA='`,
		},
		{
			name:  "ZWAGROUPMEMBER",
			query: `SELECT DISTINCT ZMEMBERJID, ZCONTACTNAME FROM ZWAGROUPMEMBER WHERE ZMEMBERJID IS NOT NULL AND ZCONTACTNAME IS NOT NULL AND ZCONTACTNAME != '' AND ZCONTACTNAME != 'IAA='`,
		},
	}

	for _, q := range queries {
		rows, err := db.QueryContext(ctx, q.query)
		if err != nil {
			s.logger.Debug("Query failed", "queryName", q.name, "error", err)
			continue
		}

		count := 0
		for rows.Next() {
			var jid, name string
			if err := rows.Scan(&jid, &name); err != nil {
				continue
			}

			// Skip empty or invalid names
			name = strings.TrimSpace(name)
			if name == "" || name == "IAA=" {
				continue
			}

			// Store the mapping
			contactMap[jid] = name

			// Also store by phone number only
			if phoneNumber := extractSenderFromJID(jid); phoneNumber != "" && phoneNumber != jid {
				// Only override if we don't have a name yet or if this is a better name
				if existingName, exists := contactMap[phoneNumber]; !exists || existingName == phoneNumber {
					contactMap[phoneNumber] = name
				}
			}
			count++
		}
		rows.Close()
		s.logger.Debug("Loaded contacts from query", "queryName", q.name, "count", count)
	}

	s.logger.Info("Loaded contact names", "totalCount", len(contactMap))
	return contactMap, nil
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

	// First, load contact names mapping
	contactNames, err := s.loadContactNames(ctx, db)
	if err != nil {
		s.logger.Warn("Failed to load contact names", "error", err)
		// Continue without contact names
		contactNames = make(map[string]string)
	}

	// First, check which columns exist in the database
	availableColumns := make(map[string]bool)
	columnQuery := `SELECT name FROM pragma_table_info('ZWAMESSAGE') WHERE name IN (
		'ZPARTICIPANTJID', 'ZSTANZAID', 'ZPUSHNAME', 'ZGROUPMEMBER'
	)`

	rows, err := db.QueryContext(ctx, columnQuery)
	if err == nil {
		for rows.Next() {
			var colName string
			if err := rows.Scan(&colName); err == nil {
				availableColumns[colName] = true
			}
		}
		rows.Close()
	}

	// Build query with proper JOIN to get group member information
	query := `SELECT 
		m.Z_PK, m.ZISFROMME, m.ZCHATSESSION, m.ZMESSAGEINFO, m.ZMESSAGEDATE, m.ZSENTDATE,
		m.ZFROMJID, m.ZTEXT, m.ZTOJID, m.ZPUSHNAME, m.ZGROUPMEMBER,
		s.ZPARTNERNAME, s.ZCONTACTJID,
		CASE 
			WHEN m.ZGROUPMEMBER IS NOT NULL THEN gm.ZMEMBERJID
			ELSE NULL
		END AS GROUPMEMBERJID,
		CASE 
			WHEN m.ZGROUPMEMBER IS NOT NULL THEN gm.ZCONTACTNAME
			ELSE NULL
		END AS GROUPMEMBERNAME
		FROM ZWAMESSAGE m
		LEFT JOIN ZWACHATSESSION s ON m.ZCHATSESSION = s.Z_PK
		LEFT JOIN ZWAGROUPMEMBER gm ON m.ZGROUPMEMBER = gm.Z_PK
		WHERE m.ZTEXT IS NOT NULL
		ORDER BY m.ZCHATSESSION, m.ZMESSAGEDATE`

	s.logger.Debug("Using query with group member join")

	rows, err = db.QueryContext(ctx, query)
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

	// Group messages by conversation
	type Message struct {
		Text            string
		IsFromMe        bool
		FromJID         string
		ToJID           string
		Timestamp       time.Time
		PartnerName     string
		ContactJID      string
		PushName        string // WhatsApp profile name
		GroupMember     string // Group member reference
		GroupMemberJID  string // Actual sender JID in group messages
		ParticipantJID  string // Alternative field for sender JID
		StanzaID        string // Message ID
		GroupMemberName string
	}

	conversationMap := make(map[string][]Message)
	participantsMap := make(map[string]map[string]bool)
	groupNamesMap := make(map[string]string) // Track group names separately

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

		for i, col := range columns {
			val := values[i]

			simplifiedKey := col
			if len(col) > 1 && col[0] == 'Z' {
				simplifiedKey = col[1:]
			}
			simplifiedKey = strings.ToLower(simplifiedKey)

			switch col {
			case "ZMESSAGEDATE", "ZSENTDATE":
				if v, ok := val.(int64); ok {
					t := convertWhatsAppTimestamp(v)
					data[simplifiedKey] = t
					if col == "ZMESSAGEDATE" {
						timestamp = t
						// Debug logging for timestamp conversion
						if t.IsZero() {
							s.logger.Warn("Failed to convert timestamp", "column", col, "rawValue", v, "type", "int64")
						} else {
							s.logger.Debug("Timestamp conversion success", "column", col, "rawValue", v, "convertedTime", t.Format(time.RFC3339))
						}
					}
				} else if v, ok := val.(float64); ok {
					t := convertWhatsAppTimestamp(v)
					data[simplifiedKey] = t
					if col == "ZMESSAGEDATE" {
						timestamp = t
						// Debug logging for timestamp conversion
						if t.IsZero() {
							s.logger.Warn("Failed to convert timestamp", "column", col, "rawValue", v, "type", "float64")
						} else {
							s.logger.Debug("Timestamp conversion success", "column", col, "rawValue", v, "convertedTime", t.Format(time.RFC3339))
						}
					}
				} else if v, ok := val.(time.Time); ok {
					// Handle time.Time values directly from database
					t := validateAndFixTimestamp(v)
					data[simplifiedKey] = t
					if col == "ZMESSAGEDATE" {
						timestamp = t
						// Debug logging for timestamp validation
						if t.IsZero() {
							s.logger.Warn("Invalid time.Time timestamp", "column", col, "rawValue", v.Format(time.RFC3339))
						} else if !t.Equal(v) {
							s.logger.Debug("Timestamp corrected", "column", col, "originalTime", v.Format(time.RFC3339), "correctedTime", t.Format(time.RFC3339))
						} else {
							s.logger.Debug("Timestamp validation success", "column", col, "time", t.Format(time.RFC3339))
						}
					}
				} else {
					data[simplifiedKey] = val
					// Debug logging for unexpected timestamp types
					s.logger.Warn("Unexpected timestamp type", "column", col, "type", fmt.Sprintf("%T", val), "value", val)
				}
			case "ZFROMJID", "ZTOJID", "ZCONTACTJID", "ZGROUPMEMBERJID", "ZPARTICIPANTJID", "GROUPMEMBERJID":
				// Handle JID fields specially - they might be stored as BLOB or encoded
				switch v := val.(type) {
				case string:
					data[simplifiedKey] = v
				case []byte:
					// iOS WhatsApp might store JIDs as NSData BLOBs
					// Try to extract readable content
					jidStr := extractJIDFromBytes(v)
					data[simplifiedKey] = jidStr
				case nil:
					data[simplifiedKey] = ""
				default:
					// Log unexpected type and convert to string
					s.logger.Debug("Unexpected JID type", "column", col, "type", fmt.Sprintf("%T", val), "value", fmt.Sprintf("%v", val))
					data[simplifiedKey] = fmt.Sprintf("%v", val)
				}
			case "ZPUSHNAME", "ZPARTNERNAME", "GROUPMEMBERNAME":
				// Handle name fields that might contain "IAA=" default value
				switch v := val.(type) {
				case string:
					if v == "IAA=" || v == "" {
						data[simplifiedKey] = ""
					} else {
						data[simplifiedKey] = v
					}
				case []byte:
					str := string(v)
					if str == "IAA=" || str == "" {
						data[simplifiedKey] = ""
					} else {
						data[simplifiedKey] = str
					}
				case nil:
					data[simplifiedKey] = ""
				default:
					strVal := fmt.Sprintf("%v", val)
					if strVal == "IAA=" {
						data[simplifiedKey] = ""
					} else {
						data[simplifiedKey] = strVal
					}
				}
			default:
				data[simplifiedKey] = val
			}
		}

		// Extract message details first
		text, _ := data["text"].(string)
		if strings.TrimSpace(text) == "" {
			continue
		}

		chatSessionInterface, ok := data["chatsession"]
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

		// Now validate timestamp after we have text and chatSession
		if timestamp.IsZero() {
			// Instead of defaulting to current time, log the issue and try fallback
			// or try to use ZSENTDATE as fallback
			if sentDate, exists := data["sentdate"]; exists {
				switch v := sentDate.(type) {
				case time.Time:
					if !v.IsZero() {
						timestamp = v
						s.logger.Debug("Using sentdate as fallback timestamp", "text", text[:min(50, len(text))])
					}
				}
			}

			// If still zero, use a reasonable fallback instead of skipping
			if timestamp.IsZero() {
				// Use a timestamp from a few years ago as fallback instead of current time
				// This preserves the message but puts it in a reasonable historical timeframe
				fallbackTime := time.Now().AddDate(-2, 0, 0) // 2 years ago
				timestamp = fallbackTime
				s.logger.Warn("Using fallback timestamp for message with invalid timestamp",
					"text", text[:min(50, len(text))],
					"chatSession", chatSession,
					"fallbackTime", fallbackTime.Format(time.RFC3339))
			}
		}

		isFromMeInterface, ok := data["isfromme"]
		if !ok {
			continue
		}

		var isFromMe bool
		switch v := isFromMeInterface.(type) {
		case int:
			isFromMe = v == 1
		case int64:
			isFromMe = v == 1
		case float64:
			isFromMe = v == 1
		case bool:
			isFromMe = v
		default:
			continue
		}

		fromJID, _ := data["fromjid"].(string)
		toJID, _ := data["tojid"].(string)
		partnerName, _ := data["partnername"].(string)
		contactJID, _ := data["contactjid"].(string)

		// These fields might not exist depending on the database schema
		pushName, _ := data["pushname"].(string)
		groupMember, _ := data["groupmember"].(string)
		groupMemberJID, _ := data["groupmemberjid"].(string)
		participantJID, _ := data["participantjid"].(string)
		groupMemberName, _ := data["groupmembername"].(string)

		// Filter out "IAA=" which is a default/null value in WhatsApp
		if partnerName == "IAA=" {
			partnerName = ""
		}
		if pushName == "IAA=" {
			pushName = ""
		}
		if groupMemberName == "IAA=" {
			groupMemberName = ""
		}

		// Decode JIDs if they're encoded
		fromJID = decodeJID(fromJID)
		toJID = decodeJID(toJID)
		contactJID = decodeJID(contactJID)
		groupMemberJID = decodeJID(groupMemberJID)
		participantJID = decodeJID(participantJID)

		// Check if this message should be filtered out
		if shouldFilterMessage(text, partnerName) {
			s.logger.Debug("Filtering out message", "text", text, "groupName", partnerName)
			continue
		}

		// For group messages, the actual sender might be in groupMemberJID
		actualSenderJID := fromJID
		isGroupChat := strings.Contains(contactJID, "@g.us") || strings.Contains(fromJID, "@g.us")

		if isGroupChat && !isFromMe {
			if groupMemberJID != "" && !strings.Contains(groupMemberJID, "@g.us") {
				actualSenderJID = groupMemberJID
				s.logger.Debug("Using groupMemberJID as sender", "groupMemberJID", groupMemberJID, "groupMemberName", groupMemberName, "fromJID", fromJID)
			} else if participantJID != "" && !strings.Contains(participantJID, "@g.us") {
				actualSenderJID = participantJID
				s.logger.Debug("Using participantJID as sender", "participantJID", participantJID, "fromJID", fromJID)
			}
		}

		// Debug logging for problematic JIDs
		if actualSenderJID != "" && (strings.Contains(actualSenderJID, "=") || len(actualSenderJID) < 10) && actualSenderJID != fromJID {
			s.logger.Debug("Suspicious sender JID after decode", "senderJID", actualSenderJID, "pushName", pushName, "groupMemberName", groupMemberName, "isFromMe", isFromMe, "text", text[:min(50, len(text))])
		}

		// Resolve @mentions in the message text
		if participantsMap[chatSession] != nil {
			text = resolveTagsInMessage(text, contactNames, participantsMap[chatSession])
		} else {
			// Create a temporary participants map for tag resolution
			tempParticipants := make(map[string]bool)
			text = resolveTagsInMessage(text, contactNames, tempParticipants)
		}

		msg := Message{
			Text:            text,
			IsFromMe:        isFromMe,
			FromJID:         actualSenderJID, // Use the actual sender JID
			ToJID:           toJID,
			Timestamp:       timestamp,
			PartnerName:     partnerName,
			ContactJID:      contactJID,
			PushName:        pushName,
			GroupMember:     groupMember,
			GroupMemberJID:  groupMemberJID,
			ParticipantJID:  participantJID,
			GroupMemberName: groupMemberName,
			StanzaID:        "", // Will be set below
		}

		// Set StanzaID if available
		if stanzaID, ok := data["stanzaid"].(string); ok {
			msg.StanzaID = stanzaID
		}

		conversationMap[chatSession] = append(conversationMap[chatSession], msg)

		// Track participants
		if participantsMap[chatSession] == nil {
			participantsMap[chatSession] = make(map[string]bool)
		}
		participantsMap[chatSession]["me"] = true

		// Check if this is a group chat (group JIDs typically contain @g.us)
		if isGroupChat {
			// Store group name
			if partnerName != "" {
				groupNamesMap[chatSession] = partnerName
			}

			// Extract individual sender from JID for group messages
			if !isFromMe && actualSenderJID != "" && !strings.Contains(actualSenderJID, "@g.us") {
				// For group messages, prioritize the group member name from JOIN
				if groupMemberName != "" && groupMemberName != "null" && !strings.Contains(groupMemberName, "=") {
					participantsMap[chatSession][groupMemberName] = true
					s.logger.Debug("Added participant from groupMemberName", "name", groupMemberName, "jid", actualSenderJID)
				} else {
					// Extract sender from JID
					s.logger.Debug("Processing group participant", "senderJID", actualSenderJID, "pushName", pushName, "text", text[:min(50, len(text))])
					senderPhone := extractSenderFromJID(actualSenderJID)
					if senderPhone != "" {
						// Try to resolve to contact name
						if contactName, exists := contactNames[senderPhone]; exists && contactName != "" && !strings.Contains(contactName, "=") {
							participantsMap[chatSession][contactName] = true
							s.logger.Debug("Resolved participant from phone", "phone", senderPhone, "name", contactName)
						} else if contactName, exists := contactNames[actualSenderJID]; exists && contactName != "" && !strings.Contains(contactName, "=") {
							participantsMap[chatSession][contactName] = true
							s.logger.Debug("Resolved participant from full JID", "jid", actualSenderJID, "name", contactName)
						} else if pushName != "" && pushName != "null" && !strings.Contains(pushName, "=") && len(pushName) > 4 {
							// Use pushName if it's valid
							participantsMap[chatSession][pushName] = true
							s.logger.Debug("Using pushName as participant", "pushName", pushName)
						} else if senderPhone != "" {
							// Fallback to phone number if no contact name found
							participantsMap[chatSession][senderPhone] = true
							s.logger.Debug("Using phone as participant", "phone", senderPhone)
						}
					} else if pushName != "" && pushName != "null" && !strings.Contains(pushName, "=") && len(pushName) > 4 {
						// If JID extraction failed but we have a valid pushName
						participantsMap[chatSession][pushName] = true
						s.logger.Debug("Using pushName as participant (no JID)", "pushName", pushName)
					}
				}
			}
		} else {
			// For individual chats, use partner name
			if partnerName != "" {
				participantsMap[chatSession][partnerName] = true
			} else if contactJID != "" {
				// Try to resolve contact name from JID if partner name is empty
				contactPhone := extractSenderFromJID(contactJID)
				if contactName, exists := contactNames[contactPhone]; exists && contactName != "" {
					participantsMap[chatSession][contactName] = true
				} else if contactName, exists := contactNames[contactJID]; exists && contactName != "" {
					participantsMap[chatSession][contactName] = true
				} else if contactPhone != "" {
					participantsMap[chatSession][contactPhone] = true
				}
			}
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during iteration: %v", err)
	}

	// Convert conversation map to records
	var records []types.Record
	for chatSession, messages := range conversationMap {
		if len(messages) == 0 {
			continue
		}

		// Sort messages by timestamp
		sort.Slice(messages, func(i, j int) bool {
			return messages[i].Timestamp.Before(messages[j].Timestamp)
		})

		// Check if this is a group chat
		isGroupChat := false
		groupName := ""
		if gName, exists := groupNamesMap[chatSession]; exists {
			isGroupChat = true
			groupName = gName
		}

		// Build conversation messages array
		conversationMessages := make([]map[string]interface{}, 0, len(messages))
		for _, msg := range messages {
			// Apply additional filtering at conversation level
			if shouldFilterMessage(msg.Text, groupName) {
				s.logger.Debug("Filtering message at conversation level", "text", msg.Text, "groupName", groupName)
				continue
			}

			speaker := "unknown"
			if msg.IsFromMe {
				speaker = "me"
			} else if isGroupChat && msg.FromJID != "" {
				// For group messages, prioritize group member name from the JOIN
				if msg.GroupMemberName != "" && msg.GroupMemberName != "null" && !strings.Contains(msg.GroupMemberName, "=") {
					speaker = msg.GroupMemberName
					s.logger.Debug("Using GroupMemberName as speaker", "name", msg.GroupMemberName, "jid", msg.FromJID)
				} else {
					// Try to extract sender from JID
					s.logger.Debug("Processing group message", "fromJID", msg.FromJID, "pushName", msg.PushName, "text", msg.Text[:min(50, len(msg.Text))])
					senderPhone := extractSenderFromJID(msg.FromJID)
					if senderPhone != "" {
						// Try to resolve to contact name
						if contactName, exists := contactNames[senderPhone]; exists && contactName != "" && !strings.Contains(contactName, "=") {
							speaker = contactName
							s.logger.Debug("Resolved speaker from phone", "phone", senderPhone, "name", contactName)
						} else if contactName, exists := contactNames[msg.FromJID]; exists && contactName != "" && !strings.Contains(contactName, "=") {
							speaker = contactName
							s.logger.Debug("Resolved speaker from full JID", "jid", msg.FromJID, "name", contactName)
						} else {
							// Fallback to phone number if no contact name found
							speaker = senderPhone
							s.logger.Debug("Using phone as speaker", "phone", senderPhone)
						}
					} else {
						// If JID extraction failed, log and try other approaches
						s.logger.Debug("Failed to extract sender from JID", "fromJID", msg.FromJID, "pushName", msg.PushName, "text", msg.Text[:min(50, len(msg.Text))])

						// Try PushName first for group messages
						if msg.PushName != "" && msg.PushName != "null" && !strings.Contains(msg.PushName, "=") && len(msg.PushName) > 4 {
							speaker = msg.PushName
							s.logger.Debug("Using PushName as speaker", "pushName", msg.PushName)
						} else if contactName, exists := contactNames[msg.FromJID]; exists && contactName != "" {
							speaker = contactName
						} else if msg.PartnerName != "" {
							speaker = msg.PartnerName
						}
					}
				}
			} else if isGroupChat && msg.PushName != "" && msg.PushName != "null" && !strings.Contains(msg.PushName, "=") && len(msg.PushName) > 4 {
				// For group messages without FromJID but with PushName
				speaker = msg.PushName
				s.logger.Debug("Using PushName for group message without JID", "pushName", msg.PushName)
			} else if msg.PartnerName != "" {
				// For individual chats, use partner name
				speaker = msg.PartnerName
			} else if msg.ContactJID != "" {
				// Fallback: try to resolve from contact JID
				contactPhone := extractSenderFromJID(msg.ContactJID)
				if contactPhone != "" {
					if contactName, exists := contactNames[contactPhone]; exists && contactName != "" {
						speaker = contactName
					} else if contactName, exists := contactNames[msg.ContactJID]; exists && contactName != "" {
						speaker = contactName
					} else {
						speaker = contactPhone
					}
				}
			}

			// Final validation - if speaker still looks corrupted, use "unknown"
			if strings.Contains(speaker, "=") || (speaker != "me" && speaker != "unknown" && len(speaker) < 3) {
				s.logger.Debug("Corrupted speaker name detected", "speaker", speaker, "fromJID", msg.FromJID, "pushName", msg.PushName)
				// Try one more time with pushName if it's valid
				if msg.PushName != "" && msg.PushName != "null" && !strings.Contains(msg.PushName, "=") && len(msg.PushName) > 4 {
					speaker = msg.PushName
				} else {
					speaker = "unknown"
				}
			}

			// Apply final tag resolution with current participants
			finalContent := resolveTagsInMessage(msg.Text, contactNames, participantsMap[chatSession])

			conversationMessages = append(conversationMessages, map[string]interface{}{
				"speaker": speaker,
				"content": finalContent,
				"time":    msg.Timestamp,
			})
		}

		// Get participants
		participants := make([]string, 0, len(participantsMap[chatSession]))
		for participant := range participantsMap[chatSession] {
			participants = append(participants, participant)
		}

		// Use the timestamp of the first message as the conversation timestamp
		conversationTimestamp := messages[0].Timestamp

		recordData := map[string]interface{}{
			"id":           fmt.Sprintf("whatsapp-chat-%s", chatSession),
			"source":       "whatsapp",
			"chat_session": chatSession,
			"conversation": conversationMessages,
			"people":       participants,
			"user":         "me",
			"type":         "conversation",
		}

		// Add group metadata if this is a group chat
		if isGroupChat && groupName != "" {
			recordData["group_name"] = groupName
			recordData["is_group_chat"] = true
		}

		record := types.Record{
			Data:      recordData,
			Timestamp: conversationTimestamp,
			Source:    "whatsapp",
		}

		records = append(records, record)
	}

	return records, nil
}

func (s *WhatsappProcessor) ProcessDirectory(ctx context.Context, filePath string) ([]types.Record, error) {
	return nil, fmt.Errorf("sync operation not supported for WhatsApp")
}

func (s *WhatsappProcessor) ProcessFile(ctx context.Context, filePath string) ([]types.Record, error) {
	// Store is not required for reading WhatsApp database files
	// ReadWhatsAppDB only uses the logger for warnings
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

	var conversationDocuments []memory.ConversationDocument

	for _, record := range records { // Each record now represents a full conversation
		// s.logger.Info("Processing record", "record", fmt.Sprintf("%+v", record))

		id, _ := record.Data["id"].(string)
		chatSession, _ := record.Data["chat_session"].(string)
		user, _ := record.Data["user"].(string)

		// Handle people field which might be []interface{} or []string
		var people []string
		switch p := record.Data["people"].(type) {
		case []string:
			people = p
		case []interface{}:
			for _, person := range p {
				if str, ok := person.(string); ok {
					people = append(people, str)
				}
			}
		}

		// Handle conversation field
		var conversationInterface []interface{}
		switch c := record.Data["conversation"].(type) {
		case []map[string]interface{}:
			// Convert to []interface{}
			conversationInterface = make([]interface{}, len(c))
			for i, msg := range c {
				conversationInterface[i] = msg
			}
		case []interface{}:
			conversationInterface = c
		}

		if id == "" || len(conversationInterface) == 0 {
			s.logger.Debug("Skipping record", "id", id, "conversation_length", len(conversationInterface))
			continue
		}

		// Convert conversation messages
		var conversationMessages []memory.ConversationMessage
		for _, msgInterface := range conversationInterface {
			msg, ok := msgInterface.(map[string]interface{})
			if !ok {
				s.logger.Debug("Skipping non-map message", "type", fmt.Sprintf("%T", msgInterface))
				continue
			}

			speaker, _ := msg["speaker"].(string)
			content, _ := msg["content"].(string)

			var timestamp time.Time
			switch t := msg["time"].(type) {
			case time.Time:
				timestamp = t
			case string:
				parsedTime, err := time.Parse(time.RFC3339, t)
				if err == nil {
					timestamp = parsedTime
				} else {
					s.logger.Debug("Failed to parse time", "time", t, "error", err)
				}
			}

			if speaker != "" && content != "" {
				conversationMessages = append(conversationMessages, memory.ConversationMessage{
					Speaker: speaker,
					Content: content,
					Time:    timestamp,
				})
			}
		}

		if len(conversationMessages) == 0 {
			s.logger.Debug("No valid messages in conversation", "id", id)
			continue
		}

		// Build metadata
		metadata := map[string]string{
			"type":         "conversation",
			"chat_session": chatSession,
		}

		// Add group metadata if present
		if groupName, ok := record.Data["group_name"].(string); ok && groupName != "" {
			metadata["group_name"] = groupName
		}
		if isGroup, ok := record.Data["is_group_chat"].(bool); ok && isGroup {
			metadata["is_group_chat"] = "true"
		}

		conversationDoc := memory.ConversationDocument{
			FieldID:       id,
			FieldSource:   "whatsapp",
			FieldTags:     []string{"conversation", "chat"},
			People:        people,
			User:          user,
			Conversation:  conversationMessages,
			FieldMetadata: metadata,
		}

		conversationDocuments = append(conversationDocuments, conversationDoc)
	}

	s.logger.Info("Converted to documents", "documents", len(conversationDocuments))
	var documents []memory.Document
	for _, conversationDoc := range conversationDocuments {
		documents = append(documents, &conversationDoc)
	}
	return documents, nil
}

// extractJIDFromBytes attempts to extract a JID from byte data
// iOS WhatsApp stores JIDs as NSData BLOBs with specific encoding
func extractJIDFromBytes(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// Try direct string conversion first
	str := string(data)
	if strings.Contains(str, "@") && !strings.Contains(str, "\x00") {
		return str
	}

	// iOS NSData might have a header, try skipping common prefixes
	// NSData format often has length prefixes or type markers
	for i := 0; i < len(data) && i < 16; i++ {
		if i+10 < len(data) {
			candidate := string(data[i:])
			// Look for @s.whatsapp.net or @g.us patterns
			if strings.Contains(candidate, "@s.whatsapp.net") || strings.Contains(candidate, "@g.us") {
				// Extract until null byte or end
				endIdx := strings.Index(candidate, "\x00")
				if endIdx > 0 {
					return candidate[:endIdx]
				}
				return candidate
			}
		}
	}

	// Try to find phone number pattern (10+ digits)
	for i := 0; i < len(data); i++ {
		if data[i] >= '0' && data[i] <= '9' {
			// Found a digit, extract the number
			start := i
			for i < len(data) && ((data[i] >= '0' && data[i] <= '9') || data[i] == '+' || data[i] == '-') {
				i++
			}
			if i-start >= 10 {
				phoneNumber := string(data[start:i])
				// Check if followed by @
				if i < len(data) && data[i] == '@' {
					// Extract the full JID
					for i < len(data) && data[i] != 0 && data[i] > 32 && data[i] < 127 {
						i++
					}
					return string(data[start:i])
				}
				return phoneNumber
			}
		}
	}

	// Last resort: return as base64 to decode later
	return base64.StdEncoding.EncodeToString(data)
}
