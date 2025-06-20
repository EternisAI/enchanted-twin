# Telegram & WhatsApp Memory Pipeline Makefile
# 
# Simplified Pipeline: Raw → ConversationDocument (X_0) → Chunks (X_1) → Facts (X_2)
#
# Usage:
#   make whatsapp   - Convert WhatsApp SQLite to ConversationDocument (X_0)
#   make telegram   - Convert Telegram JSON to ConversationDocument (X_0)
#   make chunks     - Convert ConversationDocument (X_0) to chunks (X_1)  
#   make facts      - Convert chunks (X_1) to facts (X_2)
#   make all        - Run complete pipeline (Raw → X_2)
#   make clean      - Remove all output files
#   make build      - Build the pipeline tool

TOOL_NAME = memory-processor-test
INPUT_DIR = pipeline_input
OUTPUT_DIR = pipeline_output

# Build the tool
build:
	@echo "🔨 Building pipeline tool..."
	go build -o $(TOOL_NAME) .

# Convert WhatsApp SQLite database to ConversationDocument (X_0)
whatsapp:
	@echo "📱 Converting WhatsApp SQLite to ConversationDocument (X_0)..."
	@mkdir -p $(INPUT_DIR) $(OUTPUT_DIR)
	@if [ ! -f "$(INPUT_DIR)"/*.sqlite ] && [ ! -f "$(INPUT_DIR)"/*.db ]; then \
		echo "❌ No SQLite database found in $(INPUT_DIR)/"; \
		echo "💡 Place your WhatsApp database (ChatStorage.sqlite or *.db) in $(INPUT_DIR)/"; \
		exit 1; \
	fi
	@SQLITE_FILE=$$(ls $(INPUT_DIR)/*.sqlite $(INPUT_DIR)/*.db 2>/dev/null | head -1); \
	if [ -n "$$SQLITE_FILE" ]; then \
		echo "✅ Found WhatsApp database: $$SQLITE_FILE"; \
		go run . whatsapp "$$SQLITE_FILE"; \
		echo "✅ WhatsApp X_0 ConversationDocument created: $(OUTPUT_DIR)/X_0_whatsapp.json"; \
	fi

# Convert Telegram JSON export to ConversationDocument (X_0)
telegram:
	@echo "📱 Converting Telegram JSON to ConversationDocument (X_0)..."
	@mkdir -p $(INPUT_DIR) $(OUTPUT_DIR)
	@if [ ! -f "$(INPUT_DIR)"/*.json ]; then \
		echo "❌ No JSON export found in $(INPUT_DIR)/"; \
		echo "💡 Place your Telegram export JSON file in $(INPUT_DIR)/"; \
		exit 1; \
	fi
	@JSON_FILE=$$(ls $(INPUT_DIR)/*.json 2>/dev/null | head -1); \
	if [ -n "$$JSON_FILE" ]; then \
		echo "✅ Found Telegram export: $$JSON_FILE"; \
		go run . telegram "$$JSON_FILE"; \
		echo "✅ Telegram X_0 ConversationDocument created: $(OUTPUT_DIR)/X_0_telegram.json"; \
	fi

# Convert ConversationDocument to chunks (X_0 → X_1)
chunks:
	@echo "🧩 Converting ConversationDocument to chunks (X_0 → X_1)..."
	@mkdir -p $(INPUT_DIR) $(OUTPUT_DIR)
	@if [ -f "$(OUTPUT_DIR)/X_0_whatsapp.json" ] || [ -f "$(OUTPUT_DIR)/X_0_telegram.json" ]; then \
		echo "✅ Auto-detecting X_0 ConversationDocument..."; \
		go run . --steps chunks_only; \
		echo "✅ X_1 chunks created: $(OUTPUT_DIR)/X_1_chunked_documents.json"; \
	else \
		echo "❌ No X_0 ConversationDocument file found. Run 'make whatsapp' or 'make telegram' first."; \
		exit 1; \
	fi

# Convert chunks to facts (X_1 → X_2)
facts:
	@echo "🧠 Converting chunks to facts (X_1 → X_2)..."
	@if [ ! -f "$(OUTPUT_DIR)/X_1_chunked_documents.json" ]; then \
		echo "❌ X_1_chunked_documents.json not found. Run 'make chunks' first."; \
		exit 1; \
	fi
	@go run . --steps facts_only
	@echo "✅ X_2 facts created: $(OUTPUT_DIR)/X_2_extracted_facts.json"

# Run complete pipeline - auto-detects data source
all:
	@echo "🚀 Running complete memory pipeline..."
	@mkdir -p $(INPUT_DIR) $(OUTPUT_DIR)
	@echo "🔍 Auto-detecting data source..."
	@# Check for existing X_0 files first
	@if [ -f "$(OUTPUT_DIR)/X_0_whatsapp.json" ] || [ -f "$(OUTPUT_DIR)/X_0_telegram.json" ]; then \
		echo "✅ Found existing X_0 ConversationDocument files, proceeding with pipeline..."; \
	else \
		echo "📱 Looking for raw data sources..."; \
		HAS_WHATSAPP=false; \
		HAS_TELEGRAM=false; \
		if ls $(INPUT_DIR)/*.sqlite $(INPUT_DIR)/*.db >/dev/null 2>&1; then \
			echo "✅ Found WhatsApp database"; \
			HAS_WHATSAPP=true; \
		fi; \
		if ls $(INPUT_DIR)/*.json >/dev/null 2>&1; then \
			echo "✅ Found Telegram JSON export"; \
			HAS_TELEGRAM=true; \
		fi; \
		if [ "$$HAS_WHATSAPP" = "true" ] && [ "$$HAS_TELEGRAM" = "true" ]; then \
			echo "⚠️  Both WhatsApp and Telegram data found!"; \
			echo "💡 Choose one: 'make whatsapp all' or 'make telegram all'"; \
			echo "💡 Or manually run the conversion first, then 'make all'"; \
			exit 1; \
		elif [ "$$HAS_WHATSAPP" = "true" ]; then \
			echo "🔄 Converting WhatsApp data first..."; \
			$(MAKE) whatsapp; \
		elif [ "$$HAS_TELEGRAM" = "true" ]; then \
			echo "🔄 Converting Telegram data first..."; \
			$(MAKE) telegram; \
		else \
			echo "❌ No data sources found in $(INPUT_DIR)/"; \
			echo "💡 Place either:"; \
			echo "   📱 WhatsApp: *.sqlite or *.db file"; \
			echo "   📱 Telegram: *.json export file"; \
			exit 1; \
		fi; \
	fi
	@echo "🔄 Running pipeline steps..."
	$(MAKE) chunks  
	$(MAKE) facts
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
	@if [ -f "$(OUTPUT_DIR)/X_1_chunked_documents.json" ]; then echo "✅ X_1 chunks ready"; fi
	@if [ -f "$(OUTPUT_DIR)/X_2_extracted_facts.json" ]; then echo "✅ X_2 facts ready"; fi

# Help
help:
	@echo "Simplified Memory Pipeline Commands:"
	@echo
	@echo "📱 Data Conversion (Raw → X_0 ConversationDocument):"
	@echo "  make whatsapp   - Convert WhatsApp SQLite to X_0 ConversationDocument"
	@echo "  make telegram   - Convert Telegram JSON to X_0 ConversationDocument"
	@echo
	@echo "🧠 Atomic Memory Pipeline (X_i → X_{i+1}):"
	@echo "  make chunks     - X_0 ConversationDocument → X_1 chunks"
	@echo "  make facts      - X_1 chunks → X_2 facts"
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
	@echo
	@echo "For Telegram:"
	@echo "  make telegram   # JSON → X_0 ConversationDocument"  
	@echo "  make chunks     # X_0 → X_1 chunks"
	@echo "  make facts      # X_1 → X_2 facts"
	@echo
	@echo "🔥 Clean Pipeline - No intermediate JSONL step!"

.PHONY: build whatsapp telegram chunks facts all clean status help 