# Telegram Memory Pipeline Makefile
# 
# Usage:
#   make documents  - Convert raw data (pipeline_input/) to documents (X_1)
#   make chunks     - Convert documents (X_1) to chunks (X_1')  
#   make facts      - Convert chunks (X_1') to facts (X_2)
#   make all        - Run complete pipeline (pipeline_input/ → X_2)
#   make clean      - Remove all output files
#   make build      - Build the pipeline tool

TOOL_NAME = telegram-processor-test
INPUT_DIR = pipeline_input
OUTPUT_DIR = pipeline_output

# Build the tool
build:
	@echo "🔨 Building pipeline tool..."
	go build -o $(TOOL_NAME) .

# Convert raw data to documents (X_0 → X_1)
documents: build
	@echo "📄 Converting raw data to documents..."
	@mkdir -p $(INPUT_DIR) $(OUTPUT_DIR)
	./$(TOOL_NAME) --steps basic

# Convert documents to chunks (X_1 → X_1')
chunks: build
	@echo "🧩 Converting documents to chunks..."
	@if [ ! -f "$(OUTPUT_DIR)/X_1_documents.json" ]; then \
		echo "❌ X_1_documents.json not found. Run 'make documents' first."; \
		exit 1; \
	fi
	./$(TOOL_NAME) --steps chunking

# Convert chunks to facts (X_1' → X_2)
facts: build
	@echo "🧠 Converting chunks to facts..."
	@if [ ! -f "$(OUTPUT_DIR)/X_1'_chunked_documents.json" ]; then \
		echo "❌ X_1'_chunked_documents.json not found. Run 'make chunks' first."; \
		exit 1; \
	fi
	./$(TOOL_NAME) --input dummy.json --steps facts_only

# Run complete pipeline
all: build
	@echo "🚀 Running complete pipeline..."
	@mkdir -p $(INPUT_DIR) $(OUTPUT_DIR)
	./$(TOOL_NAME) --steps extraction

# Clean all output files
clean:
	@echo "🧹 Cleaning output files..."
	rm -rf $(OUTPUT_DIR)
	rm -f $(TOOL_NAME)

# Show pipeline status
status:
	@echo "📊 Pipeline Status:"
	@echo "Input files:"
	@ls -la $(INPUT_DIR)/ 2>/dev/null || echo "  (no input files)"
	@echo
	@echo "Output files:"
	@ls -la $(OUTPUT_DIR)/ 2>/dev/null || echo "  (no output files)"

# Help
help:
	@echo "Telegram Memory Pipeline Commands:"
	@echo
	@echo "Pipeline Steps:"
	@echo "  make documents  - Convert raw data to documents (X_0 → X_1)"
	@echo "  make chunks     - Convert documents to chunks (X_1 → X_1')"
	@echo "  make facts      - Convert chunks to facts (X_1' → X_2)"
	@echo
	@echo "Utilities:"
	@echo "  make all        - Run complete pipeline"
	@echo "  make build      - Build the tool"
	@echo "  make clean      - Remove output files"
	@echo "  make status     - Show current pipeline state"
	@echo "  make help       - Show this help"
	@echo
	@echo "Setup:"
	@echo "  1. Put your telegram export file in pipeline_input/"
	@echo "  2. Run 'make documents' to start"

.PHONY: build documents chunks facts all clean status help 