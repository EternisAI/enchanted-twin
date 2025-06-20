# Memory Pipeline Tester

A command-line tool for testing and debugging the **exact memory ingestion pipeline** used by the Enchanted Twin application. This tool processes **WhatsApp** and **Telegram** chat exports through the same atomic pipeline steps as production, with **clean polymorphic execution** for maximum debugging clarity.

## Quick Start 🚀

**The easiest way to use this tool:**

1. **Put your data file in the input folder:**
   ```bash
   # WhatsApp SQLite database
   cp ~/Downloads/whatsapp_data.sqlite pipeline_input/
   
   # OR Telegram JSON export  
   cp ~/Downloads/telegram_export.json pipeline_input/
   ```

2. **Convert to ConversationDocument (X_0):**
   ```bash
   make whatsapp    # SQLite → X_0_whatsapp.json
   # OR
   make telegram    # JSON → X_0_telegram.json
   ```

3. **Run the atomic pipeline steps:**
   ```bash
   make chunks      # X_0 → X_1 (ConversationDocument → Chunks)  
   make facts       # X_1 → X_2 (Chunks → Facts)
   ```

4. **Check results:**
   ```bash
   make status
   ls pipeline_output/X_*
   ```

**That's it!** 🎉 Clean atomic steps, no intermediate formats.

## Purpose

This tool helps developers:
- **Debug memory pipeline issues** by replicating exact production processing
- **Inspect intermediate outputs** at each atomic stage (X_0, X_1, X_2)
- **Test configuration changes** without affecting the main application
- **Validate fact extraction** from personal conversation data
- **Understand the memory workflow** through detailed logging
- **Support multiple data sources** (WhatsApp, Telegram) through clean polymorphic interface

## Clean Architecture

The tool implements a simplified architecture where each step depends ONLY on the previous step's output:

```
WhatsApp SQLite ──make whatsapp──→ X_0_whatsapp.json ┐
                                                     ├──→ X_1_chunked_documents.json
Telegram JSON ────make telegram──→ X_0_telegram.json ┘        ↓ make chunks
                                                         X_1_chunked_documents.json
                                                               ↓ make facts
                                                         X_2_extracted_facts.json
```

### Atomic Pipeline Steps

| Step | Input | Output | Description | Command |
|------|-------|--------|-------------|---------|
| **WhatsApp Conversion** | SQLite DB | `X_0_whatsapp.json` | Extract conversations to ConversationDocument | `make whatsapp` |
| **Telegram Conversion** | JSON export | `X_0_telegram.json` | Convert export to ConversationDocument | `make telegram` |
| **Chunks** | `X_0_*.json` | `X_1_chunked_documents.json` | Split conversations into manageable chunks | `make chunks` |
| **Facts** | `X_1_chunked_documents.json` | `X_2_extracted_facts.json` | Extract meaningful facts using LLM | `make facts` |

## Makefile Commands

| Command | Description | Input Required | Output |
|---------|-------------|----------------|--------|
| `make whatsapp` | Convert WhatsApp SQLite to X_0 | SQLite in `pipeline_input/` | `X_0_whatsapp.json` |
| `make telegram` | Convert Telegram JSON to X_0 | JSON in `pipeline_input/` | `X_0_telegram.json` |
| `make chunks` | X_0 → X_1 (documents to chunks) | `X_0_*.json` | `X_1_chunked_documents.json` |
| `make facts` | X_1 → X_2 (chunks to facts) | `X_1_chunked_documents.json` + API key | `X_2_extracted_facts.json` |
| `make status` | Show current pipeline state | - | Status display |
| `make clean` | Remove all output files | - | Clean slate |
| `make help` | Show all commands | - | Help text |

## Prerequisites

### Required Files
- **WhatsApp SQLite database** (from WhatsApp export) 
- **OR Telegram JSON export** (from Telegram Desktop)
- **`.env` file** in the project root (`backend/golang/.env`) with API keys

### Environment Variables

Create a `.env` file in `backend/golang/` with:

```bash
# Required for fact extraction (OpenRouter recommended)
COMPLETIONS_API_KEY=sk-or-v1-your-openrouter-api-key
COMPLETIONS_API_URL=https://openrouter.ai/api/v1
COMPLETIONS_MODEL=openai/gpt-4.1

# Optional: for embeddings (future memory storage)
EMBEDDINGS_API_KEY=sk-your-openai-api-key  
EMBEDDINGS_API_URL=https://api.openai.com/v1
EMBEDDINGS_MODEL=text-embedding-3-small

# Optional: for Weaviate (future memory storage)
WEAVIATE_PORT=51414
```

## Installation & Setup

1. **Navigate to the tool directory:**
   ```bash
   cd backend/golang/cmd/memory-processor-test
   ```

2. **Setup directories and build:**
   ```bash
   make help  # Auto-builds and shows commands
   ```

3. **Add your data:**
   ```bash
   # WhatsApp database
   cp ~/Downloads/whatsapp_data.sqlite pipeline_input/
   
   # OR Telegram export
   cp ~/Downloads/telegram_export.json pipeline_input/
   ```

4. **You're ready to go!**

## Usage Examples

### 1. WhatsApp Complete Pipeline
```bash
# Setup: Copy WhatsApp database
cp ~/Downloads/whatsapp_data.sqlite pipeline_input/

# Step 1: Convert to ConversationDocument
make whatsapp
# Output: X_0_whatsapp.json (86 conversations, 5,351 messages)

# Step 2-3: Process through pipeline  
make chunks     # X_0 → X_1
make facts      # X_1 → X_2

# Check results
make status
```

### 2. Telegram Complete Pipeline
```bash
# Setup: Copy Telegram export
cp ~/Downloads/telegram_export.json pipeline_input/

# Step 1: Convert to ConversationDocument
make telegram
# Output: X_0_telegram.json (1 conversation, 1,077 messages)

# Step 2-3: Process through pipeline
make chunks     # X_0 → X_1
make facts      # X_1 → X_2

# Check results
make status
```

### 3. Atomic Step Testing
```bash
# Test only fact extraction (after fixing extraction logic)
make facts

# Test only chunking (after changing chunk size)
make chunks

# Check what files exist
make status

# Start completely over
make clean
make whatsapp
make chunks
make facts
```

### 4. Advanced: CLI Direct Usage
```bash
# Convert WhatsApp directly (auto-detects from pipeline_input/)
./memory-processor-test whatsapp

# Convert Telegram directly (auto-detects from pipeline_input/)
./memory-processor-test telegram

# Run atomic steps directly
./memory-processor-test --steps chunks_only
./memory-processor-test --steps facts_only
```

## Output Files

### X_0_whatsapp.json / X_0_telegram.json
**ConversationDocument format** directly from processors:
```json
[
  {
    "id": "whatsapp-chat-95",
    "conversation": [...],
    "user": "User",
    "people": ["User", "Friend"],
    "metadata": {...}
  }
]
```

### X_1_chunked_documents.json  
**Document chunks** optimized for LLM processing:
```json
{
  "chunked_documents": [
    {
      "id": "whatsapp-chat-95-chunk-1",
      "conversation": [...],
      "chunk_index": 1,
      "metadata": {...}
    }
  ],
  "metadata": {
    "processed_at": "2024-06-19T11:44:00Z",
    "step": "document_to_chunks",
    "original_count": 1,
    "chunked_count": 1
  }
}
```

### X_2_extracted_facts.json
**Memory facts** extracted by LLM:
```json
{
  "facts": [
    {
      "id": "fact-uuid-1",
      "content": "User frequently discusses AI research topics",
      "source_document_id": "whatsapp-chat-95",
      "confidence": 0.9,
      "created_at": "2024-06-19T11:46:00Z"
    },
    {
      "id": "fact-uuid-2", 
      "content": "User prefers technical discussions over casual chat",
      "source_document_id": "whatsapp-chat-95",
      "confidence": 0.8,
      "created_at": "2024-06-19T11:46:01Z"
    }
  ],
  "metadata": {
    "processed_at": "2024-06-19T11:46:02Z",
    "step": "chunks_to_facts",
    "documents_count": 1,
    "facts_count": 2,
    "completions_model": "anthropic/claude-3.5-sonnet",
    "source": "real_llm_extraction_from_user_data"
  }
}
```

## What to Expect

### Typical Processing Times
- **`make whatsapp`**: 3-5 seconds (auto-detects SQLite files)
- **`make telegram`**: 2-3 seconds (auto-detects JSON files)
- **`make chunks`**: 1-2 seconds (auto-detects X_0 files)
- **`make facts`**: 30-60 seconds (depends on API speed)

### Typical Data Volumes

**WhatsApp Processing:**
- **Input**: 5.9MB SQLite database
- **X_0**: 2.1KB ConversationDocument JSON (1 document after filtering)
- **X_1**: 2.0KB chunks JSON (1 chunk)
- **X_2**: 1.2KB facts JSON (2 facts extracted)

**Telegram Processing:**
- **Input**: 708KB JSON export  
- **X_0**: 224KB ConversationDocument JSON (1 conversation, 1,077 messages)
- **X_1**: Multiple chunks for large conversations
- **X_2**: 20-40 facts typically extracted

### Console Output
Clean atomic step execution:
```bash
❯ make whatsapp
🔨 Building pipeline tool...
📱 Converting WhatsApp SQLite to ConversationDocument (X_0)...
✅ Found WhatsApp database: pipeline_input/whatsapp_data.sqlite
✅ WhatsApp X_0 ConversationDocument created: pipeline_output/X_0_whatsapp.json

❯ make chunks  
🔨 Building pipeline tool...
🧩 Converting ConversationDocument to chunks (X_0 → X_1)...
✅ Using WhatsApp X_0 ConversationDocument
✅ Chunked documents: 1 → 1 chunks

❯ make facts
🔨 Building pipeline tool...
🧠 Converting chunks to facts (X_1 → X_2)...
✅ Extracted facts: 1 chunks → 2 facts
✅ Pipeline completed successfully! 🎉
```

## Troubleshooting

### Common Issues

#### 1. "No SQLite/JSON export found"
**Cause**: No input file in `pipeline_input/`
**Solution**: 
```bash
# For WhatsApp
cp ~/Downloads/whatsapp_data.sqlite pipeline_input/

# For Telegram
cp ~/Downloads/telegram_export.json pipeline_input/
```

#### 2. "fact extraction requires COMPLETIONS_API_KEY"
**Cause**: Missing API key in `.env` file
**Solution**: 
- Add `COMPLETIONS_API_KEY=your-key` to `backend/golang/.env`
- Ensure the `.env` file is in the correct location (project root)

#### 3. "No X_0 ConversationDocument file found"
**Cause**: Running steps out of order
**Solution**: 
```bash
# Run atomic steps in order
make whatsapp    # First: create X_0
make chunks      # Second: X_0 → X_1
make facts       # Third: X_1 → X_2
```

#### 4. "Both WhatsApp and Telegram X_0 files found"
**Cause**: Multiple X_0 files exist, auto-using most recent
**Solution**: This is normal behavior. The tool automatically uses the most recently created X_0 file.

#### 5. Empty facts extracted
**Cause**: May be normal for some conversation types
**Check**: 
- Look at `X_0_*.json` content
- Verify conversations have meaningful content beyond contact lists

### Debug Mode

For additional debugging:

1. **Check current state**:
   ```bash
   make status
   ```

2. **Inspect intermediate files**:
   ```bash
   cat pipeline_output/X_0_whatsapp.json | head -20
   cat pipeline_output/X_1_chunked_documents.json | jq '.metadata'
   cat pipeline_output/X_2_extracted_facts.json | jq '.facts | length'
   ```

3. **Validate API connectivity**:
   ```bash
   curl -H "Authorization: Bearer $COMPLETIONS_API_KEY" \
        https://openrouter.ai/api/v1/models
   ```

4. **Clean slate testing**:
   ```bash
   make clean
   make whatsapp
   make chunks
   make facts
   ```

## Data Source Setup

### WhatsApp Export
1. **Export from WhatsApp** (varies by platform)
2. **Locate SQLite database** (usually `whatsapp_data.sqlite`)
3. **Copy to input folder**: `cp whatsapp_data.sqlite pipeline_input/`

### Telegram Export  
1. **Open Telegram Desktop**
2. **Go to Settings** → **Advanced** → **Export Telegram data**
3. **Select** "Personal chats" only (uncheck everything else)
4. **Choose JSON format**
5. **Export** to get `result.json`
6. **Copy to input folder**: `cp result.json pipeline_input/telegram_export.json`

## Production Alignment

This tool uses the **exact same code paths** as production:
- `whatsapp.NewWhatsAppProcessor()` - Same WhatsApp parsing
- `telegram.NewTelegramProcessor()` - Same Telegram parsing  
- `processor.ProcessFile()` - Same direct ConversationDocument conversion
- `doc.Chunk()` - Same chunking algorithm
- `evolvingmemory.ExtractFactsFromDocument()` - Same fact extraction

**Clean Pipeline**: Each step reads ONLY from the previous step's output file, ensuring mathematical purity and debugging clarity.

## API Costs

**OpenRouter pricing** (as of 2024):
- Claude 3.5 Sonnet: ~$0.01-0.03 per fact extraction call
- Typical WhatsApp run: ~$0.02-0.05 total
- Typical Telegram run: ~$0.10-0.30 total  
- Small cost for debugging, much cheaper than production issues

## Future Enhancements
- [ ] Memory storage testing (X_2 → X_3)
- [ ] Query testing (X_3 → X_4)  
- [ ] Gmail/Slack/X data source support
- [ ] Batch processing for multiple files
- [ ] Performance benchmarking
- [ ] Integration tests with production pipeline

---

**Happy debugging!** 🚀 This tool provides clean atomic step execution for understanding and debugging the memory pipeline with real user data. 