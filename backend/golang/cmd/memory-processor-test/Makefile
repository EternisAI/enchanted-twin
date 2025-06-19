# Telegram & WhatsApp Memory Pipeline Makefile
# 
# Usage:
#   make whatsapp   - Convert WhatsApp SQLite to JSONL (X_0)
#   make telegram   - Convert Telegram JSON to JSONL (X_0)
#   make documents  - Convert JSONL (X_0) to documents (X_1)
#   make chunks     - Convert documents (X_1) to chunks (X_1')  
#   make facts      - Convert chunks (X_1') to facts (X_2)
#   make all        - Run complete pipeline (X_0 → X_2)
#   make clean      - Remove all output files
#   make build      - Build the pipeline tool

TOOL_NAME = memory-processor-test
INPUT_DIR = pipeline_input
OUTPUT_DIR = pipeline_output

# Build the tool
build:
	@echo "🔨 Building pipeline tool..."
	go build -o $(TOOL_NAME) .

# Convert WhatsApp SQLite database to JSONL (X_0)
whatsapp: build
	@echo "📱 Converting WhatsApp SQLite to JSONL (X_0)..."
	@mkdir -p $(INPUT_DIR) $(OUTPUT_DIR)
	@if [ ! -f "$(INPUT_DIR)"/*.sqlite ] && [ ! -f "$(INPUT_DIR)"/*.db ]; then \
		echo "❌ No SQLite database found in $(INPUT_DIR)/"; \
		echo "💡 Place your WhatsApp database (ChatStorage.sqlite or *.db) in $(INPUT_DIR)/"; \
		exit 1; \
	fi
	@SQLITE_FILE=$$(ls $(INPUT_DIR)/*.sqlite $(INPUT_DIR)/*.db 2>/dev/null | head -1); \
	if [ -n "$$SQLITE_FILE" ]; then \
		echo "✅ Found WhatsApp database: $$SQLITE_FILE"; \
		./$(TOOL_NAME) whatsapp "$$SQLITE_FILE"; \
		echo "✅ WhatsApp X_0 JSONL created: $(OUTPUT_DIR)/X_0_whatsapp.jsonl"; \
	fi

# Convert Telegram JSON export to JSONL (X_0)
telegram: build
	@echo "📱 Converting Telegram JSON to JSONL (X_0)..."
	@mkdir -p $(INPUT_DIR) $(OUTPUT_DIR)
	@if [ ! -f "$(INPUT_DIR)"/*.json ]; then \
		echo "❌ No JSON export found in $(INPUT_DIR)/"; \
		echo "💡 Place your Telegram export JSON file in $(INPUT_DIR)/"; \
		exit 1; \
	fi
	@JSON_FILE=$$(ls $(INPUT_DIR)/*.json 2>/dev/null | head -1); \
	if [ -n "$$JSON_FILE" ]; then \
		echo "✅ Found Telegram export: $$JSON_FILE"; \
		./$(TOOL_NAME) telegram "$$JSON_FILE"; \
		echo "✅ Telegram X_0 JSONL created: $(OUTPUT_DIR)/X_0_telegram.jsonl"; \
	fi

# Convert JSONL (X_0) to documents (X_1)
documents: build
	@echo "📄 Converting JSONL (X_0) to documents (X_1)..."
	@mkdir -p $(INPUT_DIR) $(OUTPUT_DIR)
	# Auto-detect which X_0 JSONL file to use
	@if [ -f "$(OUTPUT_DIR)/X_0_whatsapp.jsonl" ] && [ -f "$(OUTPUT_DIR)/X_0_telegram.jsonl" ]; then \
		echo "⚠️  Both WhatsApp and Telegram X_0 files found. Using most recent..."; \
		JSONL_FILE=$$(ls -t $(OUTPUT_DIR)/X_0_*.jsonl | head -1); \
		echo "✅ Using: $$JSONL_FILE"; \
		./$(TOOL_NAME) --input "$$JSONL_FILE" --steps documents_only; \
	elif [ -f "$(OUTPUT_DIR)/X_0_whatsapp.jsonl" ]; then \
		echo "✅ Using WhatsApp X_0 JSONL"; \
		./$(TOOL_NAME) --input "$(OUTPUT_DIR)/X_0_whatsapp.jsonl" --steps documents_only; \
	elif [ -f "$(OUTPUT_DIR)/X_0_telegram.jsonl" ]; then \
		echo "✅ Using Telegram X_0 JSONL"; \
		./$(TOOL_NAME) --input "$(OUTPUT_DIR)/X_0_telegram.jsonl" --steps documents_only; \
	else \
		echo "❌ No X_0 JSONL file found. Run 'make whatsapp' or 'make telegram' first."; \
		exit 1; \
	fi

# Convert documents to chunks (X_1 → X_1')
chunks: build
	@echo "🧩 Converting documents to chunks..."
	@if [ ! -f "$(OUTPUT_DIR)/X_1_documents.json" ]; then \
		echo "❌ X_1_documents.json not found. Run 'make documents' first."; \
		exit 1; \
	fi
	./$(TOOL_NAME) --steps chunks_only

# Convert chunks to facts (X_1' → X_2)
facts: build
	@echo "🧠 Converting chunks to facts..."
	@if [ ! -f "$(OUTPUT_DIR)/X_1'_chunked_documents.json" ]; then \
		echo "❌ X_1'_chunked_documents.json not found. Run 'make chunks' first."; \
		exit 1; \
	fi
	./$(TOOL_NAME) --steps facts_only

# Clean all output files
clean:
	@echo "🧹 Cleaning output files..."
	rm -rf $(OUTPUT_DIR)
	rm -f $(TOOL_NAME)

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
	@if [ -f "$(OUTPUT_DIR)/X_0_whatsapp.jsonl" ]; then echo "✅ WhatsApp X_0 ready"; fi
	@if [ -f "$(OUTPUT_DIR)/X_0_telegram.jsonl" ]; then echo "✅ Telegram X_0 ready"; fi
	@if [ -f "$(OUTPUT_DIR)/X_1_documents.json" ]; then echo "✅ X_1 documents ready"; fi
	@if [ -f "$(OUTPUT_DIR)/X_1'_chunked_documents.json" ]; then echo "✅ X_1' chunks ready"; fi
	@if [ -f "$(OUTPUT_DIR)/X_2_extracted_facts.json" ]; then echo "✅ X_2 facts ready"; fi

# Help
help:
	@echo "Pure Atomic Memory Pipeline Commands:"
	@echo
	@echo "📱 Data Conversion (Raw → X_0 JSONL):"
	@echo "  make whatsapp   - Convert WhatsApp SQLite to X_0 JSONL"
	@echo "  make telegram   - Convert Telegram JSON to X_0 JSONL"
	@echo
	@echo "🧠 Atomic Memory Pipeline (X_i → X_{i+1}):"
	@echo "  make documents  - X_0 JSONL → X_1 documents"
	@echo "  make chunks     - X_1 documents → X_1' chunks"
	@echo "  make facts      - X_1' chunks → X_2 facts"
	@echo
	@echo "⚡ Utilities:"
	@echo "  make all        - Run atomic sequence: whatsapp + documents + chunks + facts"
	@echo "  make build      - Build the tool"
	@echo "  make clean      - Remove output files"
	@echo "  make status     - Show current pipeline state"
	@echo "  make help       - Show this help"
	@echo
	@echo "📋 Pure Atomic Workflow:"
	@echo
	@echo "For WhatsApp:"
	@echo "  make whatsapp   # SQLite → X_0"
	@echo "  make documents  # X_0 → X_1"
	@echo "  make chunks     # X_1 → X_1'"
	@echo "  make facts      # X_1' → X_2"
	@echo
	@echo "For Telegram:"
	@echo "  make telegram   # JSON → X_0"  
	@echo "  make documents  # X_0 → X_1"
	@echo "  make chunks     # X_1 → X_1'"
	@echo "  make facts      # X_1' → X_2"
	@echo
	@echo "🔥 Pure Markov Chain - Each step depends ONLY on previous step!"

.PHONY: build whatsapp telegram documents chunks facts all clean status help 