package constants

// ProcessorType represents the type of data processor.
type ProcessorType string

// Processor name constants.
const (
	// New format processors (store documents directly in memory).
	ProcessorTelegram       ProcessorType = "telegram"
	ProcessorWhatsapp       ProcessorType = "whatsapp"
	ProcessorGmail          ProcessorType = "gmail"
	ProcessorChatGPT        ProcessorType = "chatgpt"
	ProcessorSyncedDocument ProcessorType = "synced-document"

	// Old format processors (create JSONL files).
	ProcessorSlack ProcessorType = "slack"
	ProcessorX     ProcessorType = "x"
)

// String returns the string representation of the processor type.
func (p ProcessorType) String() string {
	return string(p)
}

// IsNewFormatProcessor checks if a processor uses the new format (direct memory storage).
func IsNewFormatProcessor(processorName string) bool {
	switch ProcessorType(processorName) {
	case ProcessorTelegram, ProcessorWhatsapp, ProcessorGmail, ProcessorChatGPT, ProcessorSyncedDocument:
		return true
	default:
		return false
	}
}
