# Telegram Memory Pipeline Tester

A command-line tool for testing and debugging the memory ingestion pipeline used by the Enchanted Twin application. This tool processes Telegram chat exports through the exact same pipeline as the main application, allowing you to debug issues and inspect intermediate outputs.

## Quick Start 🚀

**The easiest way to use this tool:**

1. **Put your data file in the input folder:**
   ```bash
   cp ~/Downloads/telegram_export.json pipeline_input/
   ```

2. **Run the pipeline step by step:**
   ```bash
   make documents  # Raw data → Clean documents (X_1)
   make chunks     # Documents → Chunks (X_1')  
   make facts      # Chunks → Facts (X_2)
   ```

3. **Or run everything at once:**
   ```bash
   make all
   ```

4. **Check results:**
   ```bash
   make status
   ls pipeline_output/
   ```

**That's it!** 🎉 No file paths, no complex commands.

## Purpose

This tool helps developers:
- **Debug memory pipeline issues** in production by replicating the exact same processing steps
- **Inspect intermediate outputs** at each stage of the pipeline
- **Test configuration changes** without affecting the main application
- **Validate fact extraction** from personal conversation data
- **Understand the memory processing workflow** through detailed logging

## Pipeline Overview

The tool processes data through a multi-stage pipeline that mirrors the production memory system:

```
X_0 (Raw Telegram JSON) 
  ↓ data_to_document
X_1 (Clean Documents) 
  ↓ document_to_chunks  
X_1' (Document Chunks)
  ↓ chunks_to_facts
X_2 (Memory Facts)
  ↓ [Future: store_memory]
X_3 (Vector Database)
```

### Pipeline Steps

| Step | Input | Output | Description |
|------|-------|--------|-------------|
| **data_to_document** | Raw Telegram JSON | Document objects | Parses messages, filters contacts, creates conversation documents |
| **document_to_chunks** | Documents | Document chunks | Splits large conversations into manageable chunks |
| **chunks_to_facts** | Document chunks | Memory facts | Uses LLM to extract meaningful facts from conversations |

## Makefile Commands

| Command | Description | Requirements |
|---------|-------------|--------------|
| `make documents` | X_0 → X_1 (Raw data to documents) | Input file in `pipeline_input/` |
| `make chunks` | X_1 → X_1' (Documents to chunks) | Existing X_1 file |
| `make facts` | X_1' → X_2 (Chunks to facts) | Existing X_1' file + API key |
| `make all` | Complete pipeline (X_0 → X_2) | Input file + API key |
| `make status` | Show current pipeline state | - |
| `make clean` | Remove all output files | - |
| `make help` | Show all commands | - |

## Prerequisites

### Required Files
- **Telegram export file** (JSON format from Telegram Desktop)
- **`.env` file** in the project root (`backend/golang/.env`) with API keys

### Environment Variables

Create a `.env` file in `backend/golang/` with the following variables:

```bash
# Required for fact extraction (OpenRouter recommended)
COMPLETIONS_API_KEY=sk-or-v1-your-openrouter-api-key
COMPLETIONS_API_URL=https://openrouter.ai/api/v1
COMPLETIONS_MODEL=openai/gpt-4.1

# Optional: for embeddings (if implementing memory storage)
EMBEDDINGS_API_KEY=sk-your-openai-api-key  
EMBEDDINGS_API_URL=https://api.openai.com/v1
EMBEDDINGS_MODEL=text-embedding-3-small

# Optional: for Weaviate (if implementing memory storage)
WEAVIATE_PORT=51414
```

## Installation & Setup

1. **Navigate to the tool directory:**
   ```bash
   cd backend/golang/cmd/telegram-processor-test
   ```

2. **Setup directories and build:**
   ```bash
   make help  # This will auto-build and show commands
   ```

3. **Add your data:**
   ```bash
   cp ~/Downloads/telegram_export.json pipeline_input/
   ```

4. **You're ready to go!**
   ```bash
   make all
   ```

## Usage

### Basic Syntax
```bash
./telegram-processor-test [options] [input_file]
```

### Command Line Options

| Option | Description | Default |
|--------|-------------|---------|
| `--input, -i` | Input Telegram export file (required) | - |
| `--output, -o` | Output directory for results | `pipeline_output` |
| `--steps, -s` | Pipeline steps to run | `basic` |
| `--help, -h` | Show help message | - |

### Pipeline Steps Options

| Steps Value | What It Runs | Requirements |
|-------------|--------------|--------------|
| `basic` | X_0 → X_1 only | None |
| `chunking` | X_0 → X_1 → X_1' | None |
| `extraction` | X_0 → X_1 → X_1' → X_2 | `COMPLETIONS_API_KEY` |
| `facts_only` | X_1' → X_2 only | `COMPLETIONS_API_KEY` |
| `all` | All implemented steps | `COMPLETIONS_API_KEY` |

## Examples

### 1. Complete Pipeline (Recommended)
```bash
# Setup: Copy your telegram export to input folder
cp ~/Downloads/telegram_export.json pipeline_input/

# Run complete pipeline
make all

# Check results
make status
```

**Expected Output:**
- `pipeline_output/X_1_documents.json` - Clean conversation documents
- `pipeline_output/X_1'_chunked_documents.json` - Chunked documents  
- `pipeline_output/X_2_extracted_facts.json` - **Extracted memory facts**
- Typical: 650+ contacts → 1 conversation → 4 chunks → 20-30 facts

### 2. Step-by-Step Pipeline
```bash
# Put your data in the input folder
cp ~/Downloads/telegram_export.json pipeline_input/

# Step 1: Raw data to documents
make documents

# Step 2: Documents to chunks  
make chunks

# Step 3: Chunks to facts (requires API key)
make facts
```

### 3. Testing/Debugging Individual Steps
```bash
# Only extract facts (useful after changing fact extraction logic)
make facts

# Check current pipeline state
make status

# Clean everything and start over
make clean
make all
```

### 4. Advanced: Using CLI Directly
```bash
# If you prefer the original CLI interface
./telegram-processor-test --input ~/Downloads/telegram_export.json --steps extraction

# Different input/output locations
./telegram-processor-test \
  --input /path/to/your/telegram_data.json \
  --output /path/to/debug_output \
  --steps all
```

## Output Files

### X_1_documents.json
**Clean document objects** ready for memory processing:
```json
{
  "conversation_documents": [...],
  "other_documents": [...],
  "metadata": {
    "source_file": "telegram_export.json",
    "total_documents": 1,
    "conversation_count": 1,
    "other_count": 0
  }
}
```

### X_1'_chunked_documents.json  
**Document chunks** optimized for LLM processing:
```json
{
  "chunked_documents": [...],
  "metadata": {
    "original_count": 1,
    "chunked_count": 4,
    "step": "chunk_documents"
  }
}
```

### X_2_extracted_facts.json
**Memory facts** extracted by LLM:
```json
{
  "facts": [
    {
      "id": "fact-uuid",
      "content": "User mentioned they work in AI research",
      "source_document_id": "conversation-chunk-1",
      "confidence": 0.9,
      "timestamp": "2024-01-15T10:30:00Z"
    }
  ],
  "metadata": {
    "facts_count": 27,
    "completions_model": "openai/gpt-4.1"
  }
}
```

## What to Expect

### Typical Processing Times
- **`make documents`**: 1-2 seconds
- **`make chunks`**: 1-2 seconds  
- **`make facts`**: 30-60 seconds (depends on API speed)
- **`make all`**: ~1 minute total

### Typical Data Volumes
For a single Telegram chat export:
- **Input**: 1000+ messages, 650+ contacts
- **After filtering**: 1 conversation document
- **After chunking**: 4-6 chunks
- **After extraction**: 20-40 facts

### Console Output with Make Commands
The tool provides clean, step-by-step output:
```bash
❯ make documents
🔨 Building pipeline tool...
📄 Converting raw data to documents...
✅ Auto-detected input file: pipeline_input/telegram_export.json
✅ Generated documents count=1

❯ make chunks  
🧩 Converting documents to chunks...
✅ Chunked documents: 1 → 4 chunks

❯ make facts
🧠 Converting chunks to facts...
✅ Extracted facts: 4 chunks → 27 facts
✅ Pipeline completed successfully! 🎉
```

## Troubleshooting

### Common Issues

#### 1. "Input file is required" 
**Cause**: No input file found
**Solution**: 
```bash
# Put your file in the input directory
cp ~/Downloads/telegram_export.json pipeline_input/

# Or use the CLI directly
./telegram-processor-test --input ~/Downloads/telegram_export.json
```

#### 2. "fact extraction requires COMPLETIONS_API_KEY"
**Cause**: Missing API key in `.env` file
**Solution**: 
- Add `COMPLETIONS_API_KEY=your-key` to `backend/golang/.env`
- Ensure the `.env` file is in the correct location (project root)

#### 3. "DNS lookup failed" or "connection refused"
**Cause**: Incorrect API URL
**Solution**:
- For OpenRouter: `COMPLETIONS_API_URL=https://openrouter.ai/api/v1`
- For OpenAI: `COMPLETIONS_API_URL=https://api.openai.com/v1`

#### 4. "failed to read documents file"
**Cause**: Running steps out of order
**Solution**: 
```bash
# Use make commands in order
make documents  # First
make chunks     # Second  
make facts      # Third

# Or run everything at once
make all
```

#### 5. Empty or minimal facts extracted
**Cause**: May be normal for some conversation types
**Check**: 
- Look at the input document content
- Verify the conversation has meaningful content beyond contacts

### Debug Mode

For additional debugging, you can:

1. **Check environment loading**:
   ```bash
   cd ../../ && grep "COMPLETIONS_" .env
   ```

2. **Inspect intermediate files**:
   ```bash
   make status  # See all files
   cat pipeline_output/X_1_documents.json | jq '.metadata'
   ```

3. **Validate API connectivity**:
   ```bash
   curl -H "Authorization: Bearer $COMPLETIONS_API_KEY" \
        https://openrouter.ai/api/v1/models
   ```

## Development Notes

### Code Structure
- **Main logic**: `main.go` - CLI interface and pipeline orchestration
- **Pipeline steps**: Individual functions matching production code paths
- **Configuration**: Minimal config loading for testing purposes
- **Logging**: Detailed progress logging for debugging

### Production Alignment
This tool uses the **exact same code paths** as the production application:
- `telegram.NewTelegramProcessor()` - Same Telegram parsing
- `dataprocessingService.ToDocuments()` - Same document conversion  
- `doc.Chunk()` - Same chunking algorithm
- `evolvingmemory.ExtractFactsFromDocument()` - Same fact extraction

### Future Enhancements
- [ ] Memory storage testing (X_2 → X_3)
- [ ] Query testing (X_3 → X_4)  
- [ ] Batch processing for multiple files
- [ ] Performance benchmarking
- [ ] Integration with other data sources (WhatsApp, Gmail, etc.)

## Getting Telegram Export Data

To test with your own data:

1. **Open Telegram Desktop**
2. **Go to Settings** → **Advanced** → **Export Telegram data**
3. **Select** "Personal chats" only (uncheck everything else)
4. **Choose JSON format**
5. **Export** to get `result.json`
6. **Use** this file as input: `--input ~/Downloads/result.json`

## API Costs

**OpenRouter pricing** (as of 2024):
- GPT-4.1: ~$0.01-0.02 per fact extraction call
- Typical run with 4 chunks: ~$0.04-0.08 total
- Small cost for debugging, much cheaper than production issues

## Support

For issues or questions:
1. **Check logs** for specific error messages
2. **Verify `.env` configuration** 
3. **Test with smaller data files** first
4. **Compare outputs** with expected formats above

---

**Happy debugging!** 🚀 This tool should help you understand and debug the memory pipeline effectively. 