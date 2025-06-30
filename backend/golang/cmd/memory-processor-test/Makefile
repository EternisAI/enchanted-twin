# Telegram & WhatsApp Memory Pipeline Makefile
# 
# Simplified Pipeline: Raw → ConversationDocument (X_0) → Chunks (X_1) → Facts (X_2)
#
# Usage:
#   make whatsapp      - Convert WhatsApp SQLite to ConversationDocument (X_0)
#   make telegram      - Convert Telegram JSON to ConversationDocument (X_0)
#   make chatgpt       - Convert ChatGPT JSON to ConversationDocument (X_0)
#   make gmail         - Convert Gmail MBOX to ConversationDocument (X_0)
#   make gmail-senders - Analyze Gmail senders to include/exclude them from the ConversationDocument (X_0)
#   make chunks        - Convert ConversationDocument (X_0) to chunks (X_1)  
#   make facts         - Convert chunks (X_1) to facts (X_2)
#   make all           - Run complete pipeline (Raw → X_2)
#   make clean         - Remove all output files
#   make build         - Build the pipeline tool

TOOL_NAME = memory-processor-test
INPUT_DIR = pipeline_input
OUTPUT_DIR = pipeline_output

# Build the tool
build:
	@echo "🔨 Building pipeline tool..."
	go build -o $(TOOL_NAME) .

# Convert WhatsApp SQLite database to ConversationDocument (X_0)
# Usage: make whatsapp [FILE=path/to/file.sqlite]
whatsapp:
	@echo "📱 Converting WhatsApp SQLite to ConversationDocument (X_0)..."
	@mkdir -p $(INPUT_DIR) $(OUTPUT_DIR)
ifdef FILE
	@echo "📁 Using specified file: $(FILE)"
	@if [ ! -f "$(FILE)" ]; then \
		echo "❌ File not found: $(FILE)"; \
		exit 1; \
	fi
	@go run . whatsapp "$(FILE)"
	@echo "✅ WhatsApp X_0 ConversationDocument created: $(OUTPUT_DIR)/X_0_whatsapp.jsonl"
else
	@echo "🔍 Auto-detecting WhatsApp database..."
	@if [ ! -f "$(INPUT_DIR)"/*.sqlite ] && [ ! -f "$(INPUT_DIR)"/*.db ]; then \
		echo "❌ No SQLite database found in $(INPUT_DIR)/"; \
		echo "💡 Place your WhatsApp database (ChatStorage.sqlite or *.db) in $(INPUT_DIR)/"; \
		echo "💡 Or use: make whatsapp FILE=path/to/your/file.sqlite"; \
		exit 1; \
	fi
	@SQLITE_FILE=$$(ls $(INPUT_DIR)/*.sqlite $(INPUT_DIR)/*.db 2>/dev/null | head -1); \
	if [ -n "$$SQLITE_FILE" ]; then \
		echo "✅ Found WhatsApp database: $$SQLITE_FILE"; \
		go run . whatsapp "$$SQLITE_FILE"; \
		echo "✅ WhatsApp X_0 ConversationDocument created: $(OUTPUT_DIR)/X_0_whatsapp.jsonl"; \
	fi
endif

# Convert Telegram JSON export to ConversationDocument (X_0)
# Usage: make telegram [FILE=path/to/file.json]
telegram:
	@echo "📱 Converting Telegram JSON to ConversationDocument (X_0)..."
	@mkdir -p $(INPUT_DIR) $(OUTPUT_DIR)
ifdef FILE
	@echo "📁 Using specified file: $(FILE)"
	@if [ ! -f "$(FILE)" ]; then \
		echo "❌ File not found: $(FILE)"; \
		exit 1; \
	fi
	@go run . telegram "$(FILE)"
	@echo "✅ Telegram X_0 ConversationDocument created: $(OUTPUT_DIR)/X_0_telegram.jsonl"
else
	@echo "🔍 Auto-detecting Telegram JSON export..."
	@if [ ! -f "$(INPUT_DIR)"/*.json ]; then \
		echo "❌ No JSON export found in $(INPUT_DIR)/"; \
		echo "💡 Place your Telegram export JSON file in $(INPUT_DIR)/"; \
		echo "💡 Or use: make telegram FILE=path/to/your/file.json"; \
		exit 1; \
	fi
	@JSON_FILE=$$(ls $(INPUT_DIR)/*.json 2>/dev/null | head -1); \
	if [ -n "$$JSON_FILE" ]; then \
		echo "✅ Found Telegram export: $$JSON_FILE"; \
		go run . telegram "$$JSON_FILE"; \
		echo "✅ Telegram X_0 ConversationDocument created: $(OUTPUT_DIR)/X_0_telegram.jsonl"; \
	fi
endif

# Convert ChatGPT JSON export to ConversationDocument (X_0)
# Usage: make chatgpt [FILE=path/to/conversations.json]
chatgpt:
	@echo "🤖 Converting ChatGPT JSON to ConversationDocument (X_0)..."
	@mkdir -p $(INPUT_DIR) $(OUTPUT_DIR)
ifdef FILE
	@echo "📁 Using specified file: $(FILE)"
	@if [ ! -f "$(FILE)" ]; then \
		echo "❌ File not found: $(FILE)"; \
		exit 1; \
	fi
	@go run . chatgpt "$(FILE)"
	@echo "✅ ChatGPT X_0 ConversationDocument created: $(OUTPUT_DIR)/X_0_chatgpt.jsonl"
else
	@echo "🔍 Auto-detecting ChatGPT conversations.json..."
	@if [ ! -f "$(INPUT_DIR)"/conversations.json ] && [ ! -f "$(INPUT_DIR)"/*.json ]; then \
		echo "❌ No ChatGPT conversations.json found in $(INPUT_DIR)/"; \
		echo "💡 Place your ChatGPT conversations.json file in $(INPUT_DIR)/"; \
		echo "💡 Or use: make chatgpt FILE=path/to/your/conversations.json"; \
		exit 1; \
	fi
	@JSON_FILE=$$(ls $(INPUT_DIR)/conversations.json $(INPUT_DIR)/*.json 2>/dev/null | head -1); \
	if [ -n "$$JSON_FILE" ]; then \
		echo "✅ Found ChatGPT export: $$JSON_FILE"; \
		go run . chatgpt "$$JSON_FILE"; \
		echo "✅ ChatGPT X_0 ConversationDocument created: $(OUTPUT_DIR)/X_0_chatgpt.jsonl"; \
	fi
endif

# Convert Gmail MBOX to ConversationDocument (X_0)
gmail:
	@echo "📧 Converting Gmail MBOX to ConversationDocument (X_0)..."
	@mkdir -p $(INPUT_DIR) $(OUTPUT_DIR)
	@if [ ! -f "$(INPUT_DIR)"/*.mbox ]; then \
		echo "❌ No Gmail MBOX found in $(INPUT_DIR)/"; \
		echo "💡 Place your Gmail MBOX file in $(INPUT_DIR)/"; \
		exit 1; \
	fi
	@MBOX_FILE=$$(ls $(INPUT_DIR)/*.mbox 2>/dev/null | head -1); \
	if [ -n "$$MBOX_FILE" ]; then \
		echo "✅ Found Gmail MBOX: $$MBOX_FILE"; \
		go run . gmail; \
		echo "✅ Gmail X_0 ConversationDocument created: $(OUTPUT_DIR)/X_0_gmail.json"; \
	fi

# Analyze Gmail senders only
gmail-senders:
	@echo "📊 Analyzing Gmail senders..."
	@mkdir -p $(INPUT_DIR) $(OUTPUT_DIR)
	@if [ ! -f "$(INPUT_DIR)"/*.mbox ]; then \
		echo "❌ No Gmail MBOX found in $(INPUT_DIR)/"; \
		echo "💡 Place your Gmail MBOX file in $(INPUT_DIR)/"; \
		exit 1; \
	fi
	@MBOX_FILE=$$(ls $(INPUT_DIR)/*.mbox 2>/dev/null | head -1); \
	if [ -n "$$MBOX_FILE" ]; then \
		echo "✅ Found Gmail MBOX: $$MBOX_FILE"; \
		go run . gmail --senders; \
		echo "✅ Sender analysis completed: $(OUTPUT_DIR)/senders.json"; \
		echo "💡 Edit $(OUTPUT_DIR)/senders.json then run 'make gmail' again"; \
	fi

# Convert ConversationDocument to chunks (X_0 → X_1)
chunks:
	@echo "🧩 Converting ConversationDocument to chunks (X_0 → X_1)..."
	@mkdir -p $(INPUT_DIR) $(OUTPUT_DIR)
	@if [ -f "$(OUTPUT_DIR)/X_0_whatsapp.jsonl" ] || [ -f "$(OUTPUT_DIR)/X_0_telegram.jsonl" ] || [ -f "$(OUTPUT_DIR)/X_0_chatgpt.jsonl" ] || [ -f "$(OUTPUT_DIR)/X_0_gmail.jsonl" ]; then \
		echo "✅ Auto-detecting X_0 ConversationDocument..."; \
		go run . chunks; \
		echo "✅ X_1 chunks created: $(OUTPUT_DIR)/X_1_chunked_documents.jsonl"; \
	else \
		echo "❌ No X_0 ConversationDocument file found. Run 'make whatsapp', 'make telegram', 'make chatgpt', or 'make gmail' first."; \
		exit 1; \
	fi

# Convert chunks to facts (X_1 → X_2)
facts:
	@echo "🧠 Converting chunks to facts (X_1 → X_2)..."
	@if [ ! -f "$(OUTPUT_DIR)/X_1_chunked_documents.jsonl" ]; then \
		echo "❌ X_1_chunked_documents.jsonl not found. Run 'make chunks' first."; \
		exit 1; \
	fi
	@go run . facts
	@echo "✅ X_2 facts created: $(OUTPUT_DIR)/X_2_extracted_facts.jsonl"

# Store facts in Weaviate database (X_2 → Weaviate)
store:
	@echo "💾 Storing facts in Weaviate database..."
	@if [ ! -f "$(OUTPUT_DIR)/X_2_extracted_facts.jsonl" ]; then \
		echo "❌ X_2_extracted_facts.jsonl not found. Run 'make facts' first."; \
		exit 1; \
	fi
	@go run . store
	@echo "✅ Facts stored in Weaviate database"

# Run comprehensive memory consolidation for all 20 subjects (Weaviate → X_3)
consolidation:
	@echo "🧩 Running comprehensive memory consolidation for all subjects..."
	@go run . consolidation
	@echo "✅ Comprehensive consolidation report created: $(OUTPUT_DIR)/X_3_consolidation_report.json"

# Store consolidated facts back into Weaviate (X_3 → Weaviate consolidations)
store-consolidations:
	@echo "📦 Storing consolidated facts in Weaviate database..."
	@if [ ! -f "$(OUTPUT_DIR)/X_3_consolidation_report.json" ]; then \
		echo "❌ X_3_consolidation_report.json not found. Run 'make consolidation' first."; \
		exit 1; \
	fi
	@go run . store-consolidations
	@echo "✅ Consolidated facts stored in Weaviate database: $(OUTPUT_DIR)/X_4_consolidation_storage_status.json"

# Query consolidated facts vs raw facts (Interactive demonstration)
query-consolidations:
	@echo "🔍 Querying consolidated vs raw facts (demonstration)..."
	@if [ ! -f "$(OUTPUT_DIR)/X_4_consolidation_storage_status.json" ]; then \
		echo "❌ Consolidations not stored yet. Run 'make store-consolidations' first."; \
		exit 1; \
	fi
	@go run . query-consolidations
	@echo "✅ Query comparison completed: $(OUTPUT_DIR)/X_5_consolidation_query_results.json"

# Query consolidated facts with custom query (Vector search)
query:
	@echo "🔍 Vector search on consolidated facts..."
	@if [ ! -f "$(OUTPUT_DIR)/X_4_consolidation_storage_status.json" ]; then \
		echo "❌ Consolidations not stored yet. Run 'make store-consolidations' first."; \
		exit 1; \
	fi
	@if [ -z "$(QUERY)" ]; then \
		echo "❌ Query required. Usage: make query QUERY=\"your search query\""; \
		echo "💡 Example: make query QUERY=\"family relationships\""; \
		exit 1; \
	fi
	@echo "🔍 Searching for: \"$(QUERY)\""
	@QUERY="$(QUERY)" go run . query-consolidations
	@echo "✅ Vector search completed!"

# Run complete pipeline - auto-detects data source
all:
	@echo "🚀 Running complete memory pipeline..."
	@mkdir -p $(INPUT_DIR) $(OUTPUT_DIR)
	@echo "🔍 Auto-detecting data source..."
	@# Check for existing X_0 files first
	@if [ -f "$(OUTPUT_DIR)/X_0_whatsapp.json" ] || [ -f "$(OUTPUT_DIR)/X_0_telegram.json" ] || [ -f "$(OUTPUT_DIR)/X_0_chatgpt.json" ] || [ -f "$(OUTPUT_DIR)/X_0_gmail.json" ]; then \
		echo "✅ Found existing X_0 ConversationDocument files, proceeding with pipeline..."; \
	else \
		echo "📱 Looking for raw data sources..."; \
		HAS_WHATSAPP=false; \
		HAS_TELEGRAM=false; \
		HAS_CHATGPT=false; \
		HAS_GMAIL=false; \
		if ls $(INPUT_DIR)/*.sqlite $(INPUT_DIR)/*.db >/dev/null 2>&1; then \
			echo "✅ Found WhatsApp database"; \
			HAS_WHATSAPP=true; \
		fi; \
		if ls $(INPUT_DIR)/*.json >/dev/null 2>&1; then \
			echo "✅ Found JSON export (Telegram/ChatGPT)"; \
			HAS_TELEGRAM=true; \
			HAS_CHATGPT=true; \
		fi; \
		if ls $(INPUT_DIR)/*.mbox >/dev/null 2>&1; then \
			echo "✅ Found Gmail MBOX"; \
			HAS_GMAIL=true; \
		fi; \
		if [ "$$HAS_WHATSAPP" = "true" ] && [ "$$HAS_TELEGRAM" = "true" ] && [ "$$HAS_CHATGPT" = "true" ] && [ "$$HAS_GMAIL" = "true" ]; then \
			echo "⚠️  All data sources found!"; \
			echo "💡 Choose one: 'make whatsapp all', 'make telegram all', 'make chatgpt all', or 'make gmail all'"; \
			echo "💡 Or manually run the conversion first, then 'make all'"; \
			exit 1; \
		elif [ "$$HAS_WHATSAPP" = "true" ] && [ "$$HAS_TELEGRAM" = "true" ] && [ "$$HAS_CHATGPT" = "true" ]; then \
			echo "⚠️  WhatsApp, Telegram, and ChatGPT data found!"; \
			echo "💡 Choose one: 'make whatsapp all', 'make telegram all', or 'make chatgpt all'"; \
			echo "💡 Or manually run the conversion first, then 'make all'"; \
			exit 1; \
		elif [ "$$HAS_WHATSAPP" = "true" ] && [ "$$HAS_TELEGRAM" = "true" ]; then \
			echo "⚠️  WhatsApp and Telegram data found!"; \
			echo "💡 Choose one: 'make whatsapp all' or 'make telegram all'"; \
			echo "💡 Or manually run the conversion first, then 'make all'"; \
			exit 1; \
		elif [ "$$HAS_WHATSAPP" = "true" ]; then \
			echo "🔄 Converting WhatsApp data first..."; \
			$(MAKE) whatsapp; \
		elif [ "$$HAS_TELEGRAM" = "true" ]; then \
			echo "🔄 Converting Telegram data first..."; \
			$(MAKE) telegram; \
		elif [ "$$HAS_CHATGPT" = "true" ]; then \
			echo "🔄 Converting ChatGPT data first..."; \
			$(MAKE) chatgpt; \
		elif [ "$$HAS_GMAIL" = "true" ]; then \
			echo "🔄 Converting Gmail data first..."; \
			$(MAKE) gmail; \
		else \
			echo "❌ No data sources found in $(INPUT_DIR)/"; \
			echo "💡 Place either:"; \
			echo "   📱 WhatsApp: *.sqlite or *.db file"; \
			echo "   📱 Telegram: *.json export file"; \
			echo "   🤖 ChatGPT: conversations.json or *.json file"; \
			echo "   📧 Gmail: *.mbox file"; \
			exit 1; \
		fi; \
	fi
	@echo "🔄 Running pipeline steps..."
	$(MAKE) chunks  
	$(MAKE) facts
	$(MAKE) store
	$(MAKE) consolidation
	@echo "🎉 Complete pipeline finished! Check $(OUTPUT_DIR)/ for results."

# Clean all output files
clean:
	@echo "🧹 Cleaning output files..."
	rm -rf $(OUTPUT_DIR)

# Show pipeline status
status:
	@echo "📊 Pipeline Status:"
	@echo
	@echo "Input files (pipeline_input/):"
	@ls -la $(INPUT_DIR)/ 2>/dev/null || echo "  (no input files)"
	@echo
	@echo "Output files (pipeline_output/):"
	@ls -la $(OUTPUT_DIR)/ 2>/dev/null || echo "  (no output files)"
	@echo
	@if [ -f "$(OUTPUT_DIR)/X_0_whatsapp.json" ]; then echo "✅ WhatsApp X_0 ConversationDocument ready"; fi
	@if [ -f "$(OUTPUT_DIR)/X_0_telegram.json" ]; then echo "✅ Telegram X_0 ConversationDocument ready"; fi
	@if [ -f "$(OUTPUT_DIR)/X_0_chatgpt.json" ]; then echo "✅ ChatGPT X_0 ConversationDocument ready"; fi
	@if [ -f "$(OUTPUT_DIR)/X_0_gmail.json" ]; then echo "✅ Gmail X_0 ConversationDocument ready"; fi
	@if [ -f "$(OUTPUT_DIR)/X_1_chunked_documents.json" ]; then echo "✅ X_1 chunks ready"; fi
	@if [ -f "$(OUTPUT_DIR)/X_2_extracted_facts.json" ]; then echo "✅ X_2 facts ready"; fi
	@if [ -f "$(OUTPUT_DIR)/X_3_consolidation_report.json" ]; then echo "✅ X_3 consolidation report ready"; fi
	@if [ -f "$(OUTPUT_DIR)/X_4_consolidation_storage_status.json" ]; then echo "✅ X_4 consolidation storage completed"; fi
	@if [ -f "$(OUTPUT_DIR)/X_5_consolidation_query_results.json" ]; then echo "✅ X_5 consolidation query testing completed"; fi

# Help
help:
	@echo "Simplified Memory Pipeline Commands:"
	@echo
	@echo "📱 Data Conversion (Raw → X_0 ConversationDocument):"
	@echo "  make whatsapp   - Convert WhatsApp SQLite to X_0 ConversationDocument"
	@echo "  make telegram   - Convert Telegram JSON to X_0 ConversationDocument"
	@echo "  make chatgpt    - Convert ChatGPT JSON to X_0 ConversationDocument"
	@echo "  make gmail      - Convert Gmail MBOX to X_0 ConversationDocument"
	@echo
	@echo "🧠 Atomic Memory Pipeline (X_i → X_{i+1}):"
	@echo "  make chunks     - X_0 ConversationDocument → X_1 chunks"
	@echo "  make facts      - X_1 chunks → X_2 facts"
	@echo "  make store      - X_2 facts → Weaviate database"
	@echo "  make consolidation - Weaviate → X_3 comprehensive consolidation (all 20 subjects)"
	@echo "  make store-consolidations - X_3 consolidation → Weaviate consolidations storage"
	@echo "  make query-consolidations - Test consolidated vs raw fact querying"
	@echo "  make query QUERY=\"...\" - Vector search on consolidated facts"
	@echo
	@echo "⚡ Utilities:"
	@echo "  make all        - Auto-detect data source and run complete pipeline (Raw → X_2)"
	@echo "  make build      - Build the tool"
	@echo "  make clean      - Remove output files"
	@echo "  make status     - Show current pipeline state"
	@echo "  make help       - Show this help"
	@echo
	@echo "📋 Simplified Workflow:"
	@echo
	@echo "For WhatsApp:"
	@echo "  make whatsapp   # SQLite → X_0 ConversationDocument"
	@echo "  make chunks     # X_0 → X_1 chunks"
	@echo "  make facts      # X_1 → X_2 facts"
	@echo "  make store      # X_2 → Weaviate database"
	@echo "  make consolidation # Weaviate → X_3 report"
	@echo
	@echo "For Telegram:"
	@echo "  make telegram   # JSON → X_0 ConversationDocument"  
	@echo "  make chunks     # X_0 → X_1 chunks"
	@echo "  make facts      # X_1 → X_2 facts"
	@echo "  make store      # X_2 → Weaviate database"
	@echo "  make consolidation # Weaviate → X_3 report"
	@echo
	@echo "For ChatGPT:"
	@echo "  make chatgpt    # JSON → X_0 ConversationDocument"
	@echo "  make chunks     # X_0 → X_1 chunks"
	@echo "  make facts      # X_1 → X_2 facts"
	@echo "  make store      # X_2 → Weaviate database"
	@echo "  make consolidation # Weaviate → X_3 report"
	@echo
	@echo "For Gmail:"
	@echo "  make gmail      # MBOX → X_0 ConversationDocument"
	@echo "  make chunks     # X_0 → X_1 chunks"
	@echo "  make facts      # X_1 → X_2 facts"
	@echo "  make store      # X_2 → Weaviate database"
	@echo "  make consolidation # Weaviate → X_3 report"
	@echo
	@echo "🔥 Clean Pipeline - No intermediate JSONL step!"
	@echo
	@echo "💡 Comprehensive Consolidation:"
	@echo "  make consolidation  # Runs consolidation for all 20 canonical subjects"
	@echo "                      # Creates comprehensive X_3 report with all subject insights"

.PHONY: build whatsapp telegram chatgpt gmail gmail-senders chunks facts store consolidation store-consolidations query-consolidations query all clean status help 