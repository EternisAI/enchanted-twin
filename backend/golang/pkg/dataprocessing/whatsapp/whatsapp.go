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

// -----------------------------------------------------------------------------
// globals / helpers
// -----------------------------------------------------------------------------

var mentionRegex = regexp.MustCompile(`@(\+?\d{5,})`)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// decodeJID attempts to decode a JID that might be base64 encoded or compressed.
func decodeJID(jid string) string {
	if jid == "" {
		return ""
	}

	// If it already looks like a valid JID, return it
	if strings.Contains(jid, "@") && (strings.Contains(jid, "s.whatsapp.net") || strings.Contains(jid, "g.us")) {
		return jid
	}

	// Try to decode base64 encoded JIDs
	if strings.Contains(jid, "=") || !strings.Contains(jid, "@") {
		try := []func(string) ([]byte, error){
			base64.StdEncoding.DecodeString,
			base64.URLEncoding.DecodeString,
			base64.RawStdEncoding.DecodeString,
			base64.RawURLEncoding.DecodeString,
		}

		for _, fn := range try {
			if decoded, err := fn(jid); err == nil && len(decoded) > 0 {
				// Try to extract JID from decoded bytes
				decodedJID := extractJIDFromBytes(decoded)
				if decodedJID != "" && decodedJID != jid {
					return decodedJID
				}
			}
		}
	}

	return jid
}

// isPrintableString checks if a string contains only printable characters.
func isPrintableString(s string) bool {
	for _, r := range s {
		if r < 32 || r > 126 {
			return false
		}
	}
	return true
}

// extractSenderFromJID extracts the sender phone/username from a JID.
func extractSenderFromJID(jid string) string {
	if jid == "" {
		return ""
	}
	jid = strings.TrimSpace(jid)

	// Remove resource part if present (e.g., "user@domain/resource" -> "user@domain")
	if idx := strings.LastIndex(jid, "/"); idx > 0 {
		jid = jid[:idx]
	}

	// Extract the username/phone part before @
	if idx := strings.Index(jid, "@"); idx > 0 {
		phone := jid[:idx]
		// Clean up the phone number
		phone = strings.TrimPrefix(phone, "+")
		if len(phone) >= 10 && len(phone) <= 20 && !strings.Contains(phone, "=") {
			// Check if it's all digits
			allDigits := true
			for _, c := range phone {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				return phone
			}
		}
	}

	// If no @ found but looks like a phone number
	jid = strings.TrimPrefix(jid, "+")
	if len(jid) >= 10 && len(jid) <= 20 && !strings.Contains(jid, "=") {
		allDigits := true
		for _, c := range jid {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return jid
		}
	}

	return ""
}

// -----------------------------------------------------------------------------
// processor
// -----------------------------------------------------------------------------

type WhatsappProcessor struct {
	store  *db.Store
	logger *log.Logger
}

func NewWhatsappProcessor(store *db.Store, logger *log.Logger) processor.Processor {
	return &WhatsappProcessor{store: store, logger: logger}
}

func (s *WhatsappProcessor) Name() string { return "whatsapp" }

// -----------------------------------------------------------------------------
// contacts
// -----------------------------------------------------------------------------

func (s *WhatsappProcessor) loadContactNames(ctx context.Context, db *sql.DB) (map[string]string, error) {
	contactMap := make(map[string]string)

	queries := []struct {
		name  string
		query string
	}{
		{"ZWAPROFILEPUSHNAME", `SELECT ZJID, ZPUSHNAME FROM ZWAPROFILEPUSHNAME
			WHERE ZJID IS NOT NULL AND ZPUSHNAME IS NOT NULL AND ZPUSHNAME!='' AND ZPUSHNAME!='IAA='`},
		{"ZWAGROUPMEMBER", `SELECT DISTINCT ZMEMBERJID, ZCONTACTNAME FROM ZWAGROUPMEMBER
			WHERE ZMEMBERJID IS NOT NULL AND ZCONTACTNAME IS NOT NULL AND ZCONTACTNAME!='' AND ZCONTACTNAME!='IAA='`},
	}

	for _, q := range queries {
		rows, err := db.QueryContext(ctx, q.query)
		if err != nil {
			s.logger.Debug("query failed", "query", q.name, "err", err)
			continue
		}
		for rows.Next() {
			var jid, name string
			if err := rows.Scan(&jid, &name); err != nil {
				continue
			}
			name = strings.TrimSpace(name)
			if name == "" || name == "IAA=" || strings.Contains(name, "=") || len(name) < 2 {
				continue
			}

			// Don't override good data with bad data
			if existing, ok := contactMap[jid]; ok && existing != "" && !strings.Contains(existing, "=") && len(existing) >= 2 {
				continue
			}

			contactMap[jid] = name

			// Also map the phone number without domain
			if phone := extractSenderFromJID(jid); phone != "" {
				// Only add if we don't already have a better name
				if existing, ok := contactMap[phone]; !ok || existing == "" || strings.Contains(existing, "=") || len(existing) < 2 {
					contactMap[phone] = name
				}
				if existing, ok := contactMap["+"+phone]; !ok || existing == "" || strings.Contains(existing, "=") || len(existing) < 2 {
					contactMap["+"+phone] = name
				}
				if existing, ok := contactMap[phone+"@s.whatsapp.net"]; !ok || existing == "" || strings.Contains(existing, "=") || len(existing) < 2 {
					contactMap[phone+"@s.whatsapp.net"] = name
				}
			}
		}
		_ = rows.Close()
	}
	s.logger.Info("loaded contacts", "count", len(contactMap))
	return contactMap, nil
}

// -----------------------------------------------------------------------------
// db reader
// -----------------------------------------------------------------------------

func (s *WhatsappProcessor) ReadWhatsAppDB(ctx context.Context, dbPath string) ([]types.Record, error) {
	sqliteDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	defer sqliteDB.Close()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	contactNames, err := s.loadContactNames(ctx, sqliteDB)
	if err != nil {
		s.logger.Warn("load contacts", "err", err)
		contactNames = map[string]string{}
	}

	query := `SELECT 
		m.Z_PK, m.ZISFROMME, m.ZCHATSESSION, m.ZMESSAGEINFO, m.ZMESSAGEDATE, m.ZSENTDATE,
		m.ZFROMJID, m.ZTEXT, m.ZTOJID, m.ZPUSHNAME, m.ZGROUPMEMBER,
		s.ZPARTNERNAME, s.ZCONTACTJID,
		CASE WHEN m.ZGROUPMEMBER IS NOT NULL THEN gm.ZMEMBERJID END AS GROUPMEMBERJID,
		CASE WHEN m.ZGROUPMEMBER IS NOT NULL THEN gm.ZCONTACTNAME END AS GROUPMEMBERNAME
	FROM ZWAMESSAGE m
	LEFT JOIN ZWACHATSESSION s ON m.ZCHATSESSION = s.Z_PK
	LEFT JOIN ZWAGROUPMEMBER gm ON m.ZGROUPMEMBER = gm.Z_PK
	WHERE m.ZTEXT IS NOT NULL
	ORDER BY m.ZCHATSESSION, m.ZMESSAGEDATE`

	rows, err := sqliteDB.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	colCnt := len(cols)
	vals := make([]interface{}, colCnt)
	ptrs := make([]interface{}, colCnt)
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	type Message struct {
		Text            string
		IsFromMe        bool
		FromJID         string
		ToJID           string
		Timestamp       time.Time
		PartnerName     string
		ContactJID      string
		PushName        string
		GroupMemberName string
		GroupMemberJID  string
	}

	convs := map[string][]Message{}
	participants := map[string]map[string]bool{}
	groupNames := map[string]string{}

	rowIdx := 0
	for rows.Next() {
		if rowIdx%500 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		rowIdx++
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		data := map[string]interface{}{}
		var msgDate, sentDate time.Time

		for i, c := range cols {
			key := strings.ToLower(strings.TrimPrefix(c, "Z"))
			switch c {
			case "ZMESSAGEDATE", "ZSENTDATE":
				// WhatsApp on iOS uses Apple's Core Data timestamps (seconds since 2001-01-01)
				// Unix epoch is 1970-01-01, so we need to add the offset: 978307200 seconds
				const appleEpochOffset = 978307200
				switch v := vals[i].(type) {
				case int64:
					t := time.Unix(v+appleEpochOffset, 0)
					data[key] = t
					if c == "ZMESSAGEDATE" {
						msgDate = t
					} else {
						sentDate = t
					}
				case float64:
					t := time.Unix(int64(v)+appleEpochOffset, 0)
					data[key] = t
					if c == "ZMESSAGEDATE" {
						msgDate = t
					} else {
						sentDate = t
					}
				}
			case "ZFROMJID", "ZTOJID", "ZCONTACTJID", "GROUPMEMBERJID":
				switch v := vals[i].(type) {
				case string:
					data[key] = decodeJID(v)
				case []byte:
					data[key] = extractJIDFromBytes(v)
				}
			default:
				data[key] = vals[i]
			}
		}

		// Only skip messages that are clearly system messages or media without text
		if info, ok := data["messageinfo"]; ok {
			switch v := info.(type) {
			case int64:
				// Only skip specific system message types, not all non-zero values
				if v == 1 || v == 2 || v == 3 || v == 4 || v == 5 {
					continue
				}
			case int:
				// Only skip specific system message types, not all non-zero values
				if v == 1 || v == 2 || v == 3 || v == 4 || v == 5 {
					continue
				}
			}
		}

		text, _ := data["text"].(string)
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		// resolve mentions like @1234567890
		text = mentionRegex.ReplaceAllStringFunc(text, func(tag string) string {
			phone := strings.TrimPrefix(tag, "@")
			if name, ok := contactNames[phone]; ok && name != "" {
				return "@" + name
			}
			if name, ok := contactNames[decodeJID(phone+"@s.whatsapp.net")]; ok && name != "" {
				return "@" + name
			}
			return tag
		})

		chatSession := fmt.Sprintf("%v", data["chatsession"])
		isFromMe := false
		switch v := data["isfromme"].(type) {
		case int64:
			isFromMe = v == 1
		case int:
			isFromMe = v == 1
		case bool:
			isFromMe = v
		}

		// Extract JIDs more carefully
		fromJID := ""
		if jid := data["fromjid"]; jid != nil {
			fromJID = fmt.Sprintf("%v", jid)
			if decoded := decodeJID(fromJID); decoded != "" && decoded != fromJID {
				fromJID = decoded
			}
		}

		toJID := ""
		if jid := data["tojid"]; jid != nil {
			toJID = fmt.Sprintf("%v", jid)
			if decoded := decodeJID(toJID); decoded != "" && decoded != toJID {
				toJID = decoded
			}
		}

		contactJID := ""
		if jid := data["contactjid"]; jid != nil {
			contactJID = fmt.Sprintf("%v", jid)
			if decoded := decodeJID(contactJID); decoded != "" && decoded != contactJID {
				contactJID = decoded
			}
		}

		pushName, _ := data["pushname"].(string)
		partnerName, _ := data["partnername"].(string)
		groupMemberName, _ := data["groupmembername"].(string)
		groupMemberJID, _ := data["groupmemberjid"].(string)

		timestamp := sentDate
		if timestamp.IsZero() {
			timestamp = msgDate
		}
		if timestamp.IsZero() {
			timestamp = time.Now()
		}

		msg := Message{
			Text:            text,
			IsFromMe:        isFromMe,
			FromJID:         fromJID,
			ToJID:           toJID,
			Timestamp:       timestamp,
			ContactJID:      contactJID,
			PushName:        pushName,
			PartnerName:     partnerName,
			GroupMemberName: groupMemberName,
			GroupMemberJID:  groupMemberJID,
		}

		convs[chatSession] = append(convs[chatSession], msg)

		if participants[chatSession] == nil {
			participants[chatSession] = map[string]bool{"me": true}
		}
		if partnerName != "" && partnerName != "IAA=" {
			participants[chatSession][partnerName] = true
		}
		if pushName != "" && pushName != "IAA=" {
			participants[chatSession][pushName] = true
		}

		// Identify group chats
		if strings.Contains(contactJID, "@g.us") {
			if partnerName != "" && partnerName != "IAA=" {
				groupNames[chatSession] = partnerName
			} else if pushName != "" && pushName != "IAA=" {
				groupNames[chatSession] = pushName
			}
		} else if strings.Contains(toJID, "@g.us") || strings.Contains(fromJID, "@g.us") {
			// Sometimes group info is in from/to JID
			groupNames[chatSession] = "Group Chat " + chatSession
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// -------------------------------------------------------------------------
	// build records
	// -------------------------------------------------------------------------

	var records []types.Record
	for chatID, msgs := range convs {
		if len(msgs) == 0 {
			continue
		}

		sort.Slice(msgs, func(i, j int) bool { return msgs[i].Timestamp.Before(msgs[j].Timestamp) })

		msgArr := make([]map[string]interface{}, len(msgs))
		for i, m := range msgs {
			speaker := "me"
			if !m.IsFromMe {
				// For group chats, we need to get the actual sender from the group member data
				// The FromJID will be the group JID, not the individual sender

				// Priority 1: If we have group member JID, look up the contact first (most reliable)
				if m.GroupMemberJID != "" {
					memberJID := m.GroupMemberJID
					lookupKeys := []string{
						memberJID,
						extractSenderFromJID(memberJID),
						"+" + extractSenderFromJID(memberJID),
						extractSenderFromJID(memberJID) + "@s.whatsapp.net",
					}

					for _, key := range lookupKeys {
						if key != "" && key != "+" {
							if name, ok := contactNames[key]; ok && name != "" && name != "IAA=" && !strings.HasPrefix(name, "IAA") {
								speaker = name
								break
							}
						}
					}

					// If no contact name found, use the phone number from member JID
					if speaker == "me" {
						phone := extractSenderFromJID(memberJID)
						if phone != "" && len(phone) >= 10 && len(phone) <= 20 {
							allDigits := true
							for _, c := range phone {
								if c < '0' || c > '9' {
									allDigits = false
									break
								}
							}
							if allDigits {
								speaker = "+" + phone
							}
						}
					}
				}

				// Priority 2: Group member name from the join (only if it looks valid)
				if speaker == "me" && m.GroupMemberName != "" && m.GroupMemberName != "IAA=" &&
					!strings.HasPrefix(m.GroupMemberName, "IAA") && !strings.Contains(m.GroupMemberName, "=") &&
					len(m.GroupMemberName) > 2 {
					speaker = m.GroupMemberName
				}

				// Priority 3: For non-group messages, try the FromJID
				if speaker == "me" && !strings.Contains(m.FromJID, "@g.us") {
					lookupKeys := []string{
						m.FromJID,
						extractSenderFromJID(m.FromJID),
						"+" + extractSenderFromJID(m.FromJID),
						extractSenderFromJID(m.FromJID) + "@s.whatsapp.net",
					}

					for _, key := range lookupKeys {
						if key != "" && key != "+" {
							if name, ok := contactNames[key]; ok && name != "" && name != "IAA=" && !strings.HasPrefix(name, "IAA") {
								speaker = name
								break
							}
						}
					}
				}

				// Priority 4: Push name
				if speaker == "me" && m.PushName != "" && m.PushName != "IAA=" && !strings.HasPrefix(m.PushName, "IAA") {
					speaker = m.PushName
				}

				// Priority 5: Partner name (for individual chats)
				if speaker == "me" && m.PartnerName != "" && m.PartnerName != "IAA=" && !strings.HasPrefix(m.PartnerName, "IAA") && !strings.Contains(m.FromJID, "@g.us") {
					speaker = m.PartnerName
				}

				// Priority 6: Use phone number from FromJID (for individual chats)
				if speaker == "me" && !strings.Contains(m.FromJID, "@g.us") {
					phone := extractSenderFromJID(m.FromJID)
					if phone != "" && len(phone) >= 10 && len(phone) <= 20 {
						allDigits := true
						for _, c := range phone {
							if c < '0' || c > '9' {
								allDigits = false
								break
							}
						}
						if allDigits {
							speaker = "+" + phone
						}
					}
				}

				// Last resort: Use "Unknown" only if we really can't find anything
				if speaker == "me" {
					speaker = "Unknown"
				}
			}

			msgArr[i] = map[string]interface{}{
				"speaker": speaker,
				"content": m.Text,
				"time":    m.Timestamp,
			}
		}

		// Add speakers from messages to participants
		for _, msg := range msgArr {
			if speaker, ok := msg["speaker"].(string); ok && speaker != "Unknown" && speaker != "me" && speaker != "" && speaker != "IAA=" {
				participants[chatID][speaker] = true
			}
		}

		// Filter and create participants list
		parts := make([]string, 0, len(participants[chatID]))
		dedupMap := make(map[string]string) // normalized -> display name

		for p := range participants[chatID] {
			if p == "" || p == "Unknown" || strings.HasPrefix(p, "IAA") || strings.Contains(p, "=") {
				continue
			}

			// Normalize phone numbers for deduplication
			normalized := p
			if phone := extractSenderFromJID(p); phone != "" {
				normalized = phone
			}

			// Keep the better formatted version
			if existing, ok := dedupMap[normalized]; ok {
				// Prefer names over phone numbers
				if !strings.HasPrefix(existing, "+") && strings.HasPrefix(p, "+") {
					continue
				}
				// Prefer formatted phone numbers with +
				if strings.HasPrefix(existing, "+") && !strings.HasPrefix(p, "+") {
					dedupMap[normalized] = existing
					continue
				}
			}
			dedupMap[normalized] = p
		}

		// Convert deduplicated map to list
		for _, displayName := range dedupMap {
			parts = append(parts, displayName)
		}

		recData := map[string]interface{}{
			"id":           fmt.Sprintf("whatsapp-chat-%s", chatID),
			"source":       "whatsapp",
			"chat_session": chatID,
			"conversation": msgArr,
			"people":       parts,
			"user":         "me",
			"type":         "conversation",
		}
		if name, ok := groupNames[chatID]; ok {
			recData["group_name"] = name
			recData["is_group_chat"] = true
		}

		records = append(records, types.Record{
			Data:      recData,
			Timestamp: msgs[0].Timestamp,
			Source:    "whatsapp",
		})
	}
	return records, nil
}

// -----------------------------------------------------------------------------
// file / directory handlers
// -----------------------------------------------------------------------------

func (s *WhatsappProcessor) ProcessDirectory(ctx context.Context, _ string) ([]types.Record, error) {
	return nil, fmt.Errorf("sync operation not supported for WhatsApp")
}

func (s *WhatsappProcessor) ProcessFile(ctx context.Context, filePath string) ([]types.Record, error) {
	return s.ReadWhatsAppDB(ctx, filePath)
}

func (s *WhatsappProcessor) Sync(context.Context, string) ([]types.Record, bool, error) {
	return nil, false, fmt.Errorf("sync operation not supported for WhatsApp")
}

// -----------------------------------------------------------------------------
// to documents
// -----------------------------------------------------------------------------

func (s *WhatsappProcessor) ToDocuments(ctx context.Context, records []types.Record) ([]memory.Document, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if len(records) == 0 {
		return []memory.Document{}, nil
	}

	var docs []memory.Document
	for _, r := range records {
		id, _ := r.Data["id"].(string)
		chatSession, _ := r.Data["chat_session"].(string)
		user, _ := r.Data["user"].(string)

		var people []string
		switch p := r.Data["people"].(type) {
		case []string:
			people = p
		case []interface{}:
			for _, v := range p {
				if s, ok := v.(string); ok {
					people = append(people, s)
				}
			}
		}

		var convRaw []interface{}
		switch c := r.Data["conversation"].(type) {
		case []interface{}:
			convRaw = c
		case []map[string]interface{}:
			for _, m := range c {
				convRaw = append(convRaw, m)
			}
		}

		var convMsgs []memory.ConversationMessage
		for _, m := range convRaw {
			if mm, ok := m.(map[string]interface{}); ok {
				speaker, _ := mm["speaker"].(string)
				content, _ := mm["content"].(string)
				var msgTime time.Time
				switch t := mm["time"].(type) {
				case time.Time:
					msgTime = t
				case string:
					if parsed, err := time.Parse(time.RFC3339, t); err == nil {
						msgTime = parsed
					}
				}
				if speaker != "" && content != "" {
					convMsgs = append(convMsgs, memory.ConversationMessage{
						Speaker: speaker,
						Content: content,
						Time:    msgTime,
					})
				}
			}
		}

		if len(convMsgs) == 0 {
			continue
		}

		meta := map[string]string{
			"chat_session": chatSession,
			"type":         "conversation",
		}
		if g, ok := r.Data["group_name"].(string); ok && g != "" {
			meta["group_name"] = g
		}
		if ig, ok := r.Data["is_group_chat"].(bool); ok && ig {
			meta["is_group_chat"] = "true"
		}

		docs = append(docs, &memory.ConversationDocument{
			FieldID:       id,
			FieldSource:   "whatsapp",
			FieldTags:     []string{"conversation", "chat"},
			People:        people,
			User:          user,
			Conversation:  convMsgs,
			FieldMetadata: meta,
		})
	}
	s.logger.Info("converted documents", "count", len(docs))
	return docs, nil
}

// -----------------------------------------------------------------------------
// byte helpers
// -----------------------------------------------------------------------------

func extractJIDFromBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	// First, try to interpret as a simple string
	str := string(b)
	if isPrintableString(str) && strings.Contains(str, "@") {
		// Clean up any null bytes
		if idx := strings.Index(str, "\x00"); idx > 0 {
			str = str[:idx]
		}
		return str
	}

	// Look for WhatsApp JID patterns
	for i := 0; i < len(b); i++ {
		// Look for phone number patterns
		if (b[i] >= '0' && b[i] <= '9') || b[i] == '+' {
			start := i
			// Scan for phone number
			for i < len(b) && ((b[i] >= '0' && b[i] <= '9') || b[i] == '+' || b[i] == '-') {
				i++
			}

			// Check if we have a valid phone number length
			if i-start >= 10 && i-start <= 20 {
				phone := string(b[start:i])

				// Check if followed by @ (indicating JID)
				if i < len(b) && b[i] == '@' {
					jidStart := start
					// Continue reading the domain part
					for i < len(b) && b[i] != 0 && b[i] >= 32 && b[i] <= 126 {
						i++
					}
					jid := string(b[jidStart:i])
					if strings.Contains(jid, "@s.whatsapp.net") || strings.Contains(jid, "@g.us") {
						return jid
					}
				}
				// Return just the phone number if it looks valid
				return phone + "@s.whatsapp.net"
			}
		}

		// Look for @ symbol that might indicate a JID
		if b[i] == '@' && i > 0 {
			// Scan backwards to find the start
			start := i - 1
			for start > 0 && b[start] >= 32 && b[start] <= 126 && b[start] != 0 {
				start--
			}
			if b[start] < 32 || b[start] > 126 {
				start++
			}

			// Scan forwards to find the end
			end := i
			for end < len(b) && b[end] >= 32 && b[end] <= 126 && b[end] != 0 {
				end++
			}

			candidate := string(b[start:end])
			if strings.Contains(candidate, "@s.whatsapp.net") || strings.Contains(candidate, "@g.us") {
				return candidate
			}
		}
	}

	// If we can't extract a valid JID, return empty string instead of base64
	return ""
}
