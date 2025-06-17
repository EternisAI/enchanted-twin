-- WhatsApp message queries
-- name: InsertWhatsappMessage :exec
INSERT INTO whatsapp_messages (id, conversation_id, sender_jid, sender_name, content, message_type, timestamp, from_me)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetWhatsappMessagesByConversation :many
SELECT id, conversation_id, sender_jid, sender_name, content, message_type, timestamp, from_me, created_at
FROM whatsapp_messages 
WHERE conversation_id = ? 
ORDER BY timestamp ASC;

-- name: GetWhatsappMessageCount :one
SELECT COUNT(*) FROM whatsapp_messages WHERE conversation_id = ?;

-- name: GetLatestWhatsappMessage :one
SELECT id, conversation_id, sender_jid, sender_name, content, message_type, timestamp, from_me, created_at
FROM whatsapp_messages 
WHERE conversation_id = ? 
ORDER BY timestamp DESC 
LIMIT 1;

-- name: DeleteWhatsappMessagesByConversation :exec
DELETE FROM whatsapp_messages WHERE conversation_id = ?;

-- name: GetAllConversationIDs :many
SELECT DISTINCT conversation_id FROM whatsapp_messages; 