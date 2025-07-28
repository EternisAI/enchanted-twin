# Enchanted

Enchanted is a personal AI assistant.

## Run

You can download signed binary from [Github releases](https://github.com/EternisAI/enchanted-twin/releases).

## Development

Requirements

- Go
- pnpm
- Node.js
- LLM (OpenAI, OpenRouter)

### Frontend

1. Navigate to the `app` directory
1. Install packages `pnpm install`
1. Run the app `cd app && pnpm dev`

### Backend

1. Navigate to `backend/golang`
1. Copy `.env.sample` to `.env` and update env variables
1. Run the server `make run`

Common development commands:

- `make build` - Build the binary
- `make test` - Run tests
- `make lint` - Auto-fix formatting and run linters
- `make deadcode` - Check for unused code
- `make gqlgen` - Generate GraphQL resolvers

### GraphQL

Backend

On the backend side GraphQL resolvers (`schema.resolvers.go`) are code-generated from the schema `schema.graphqls`. Steps to update the schema

1. Propose schema changes in `schema.graphqls`.
1. Generate resolvers using `make gqlgen` in `backend/golang` directory.
1. This will generate additional code in `schema.resolvers.go`.

Frontend

Frontend uses `schema.graphqls` as the source of truth to code generate queries/mutations/subscriptions using `pnpm codegen`.

## Release (build installer)

Build a release for Mac M series use.

```sh
COMPLETIONS_API_URL='https://enchanted.ngrok.pro/v1' \
COMPLETIONS_MODEL='mistral-small3.1' \
REASONING_MODEL='qwen3:32b' \
EMBEDDINGS_API_URL='https://enchanted.ngrok.pro/v1' \
EMBEDDINGS_MODEL='nomic-embed-text' \
IS_PROD_BUILD='true' \
NOTARY_API_KEY_ID=742ZY9FRN6 \
NOTARY_API_ISSUER=899fdbc2-cee9-4aea-b78b-850333a61f19 \
NOTARY_TEAM_ID=JDDZ55DT74 \
TELEGRAM_CHAT_SERVER=https://enchanted-proxy-telegram-dev.up.railway.app/query \
ENCHANTED_MCP_URL=https://enchanted-proxy-dev.up.railway.app/mcp \
POSTHOG_API_KEY=phc_z8xhkNCHHUClOYiQ79nLsMeY7rxbWqCpI8KQUmmcKd8 \
TTS_API_KEY=tinfoil-api-key \
TTS_MODEL=kokoro \
TTS_URL=https://audio-processing.model.tinfoil.sh/v1/ \
STT_MODEL="whisper-large-v3-turbo" \
STT_URL=https://audio-processing.model.tinfoil.sh/v1/ \
make build-mac-silicon
```

Build a release for all architectures

```sh
COMPLETIONS_API_URL='https://enchanted.ngrok.pro/v1' \
COMPLETIONS_MODEL='mistral-small3.1' \
REASONING_MODEL='qwen3:32b' \
EMBEDDINGS_API_URL='https://enchanted.ngrok.pro/v1' \
EMBEDDINGS_MODEL='nomic-embed-text' \
IS_PROD_BUILD='true' \
NOTARY_API_KEY_ID=742ZY9FRN6 \
NOTARY_API_ISSUER=899fdbc2-cee9-4aea-b78b-850333a61f19 \
NOTARY_TEAM_ID=JDDZ55DT74 \
TELEGRAM_CHAT_SERVER=https://enchanted-proxy-telegram-dev.up.railway.app/query \
ENCHANTED_MCP_URL=https://enchanted-proxy-dev.up.railway.app/mcp \
POSTHOG_API_KEY=phc_z8xhkNCHHUClOYiQ79nLsMeY7rxbWqCpI8KQUmmcKd8 \
TTS_API_KEY=tinfoil-api-key \
TTS_MODEL=kokoro \
TTS_URL=https://audio-processing.model.tinfoil.sh/v1/ \
STT_API_KEY=tinfoil-api-key \
STT_MODEL="whisper-large-v3-turbo" \
STT_URL=https://audio-processing.model.tinfoil.sh/v1/ \
make build-all
```

Local build (Production)

```sh
COMPLETIONS_API_URL='https://enchanted.ngrok.pro/v1' \
COMPLETIONS_MODEL='mistral-small3.1' \
REASONING_MODEL='qwen3:32b' \
EMBEDDINGS_API_URL='https://enchanted.ngrok.pro/v1' \
EMBEDDINGS_MODEL='nomic-embed-text' \
IS_PROD_BUILD='true' \
NOTARY_API_KEY_ID=742ZY9FRN6 \
NOTARY_API_ISSUER=899fdbc2-cee9-4aea-b78b-850333a61f19 \
NOTARY_TEAM_ID=JDDZ55DT74 \
TELEGRAM_CHAT_SERVER=https://enchanted-proxy-telegram-dev.up.railway.app/query \
ENCHANTED_MCP_URL=https://enchanted-proxy-dev.up.railway.app/mcp \
POSTHOG_API_KEY=phc_z8xhkNCHHUClOYiQ79nLsMeY7rxbWqCpI8KQUmmcKd8 \
TTS_API_KEY=tinfoil-api-key \
TTS_MODEL=kokoro \
TTS_URL=https://audio-processing.model.tinfoil.sh/v1/ \
STT_API_KEY=tinfoil-api-key \
STT_MODEL="whisper-large-v3-turbo" \
STT_URL=https://audio-processing.model.tinfoil.sh/v1/ \
pnpm build-local:mac
```

Local build (Development)

```sh
COMPLETIONS_API_URL='https://enchanted.ngrok.pro/v1' \
COMPLETIONS_MODEL='mistral-small3.1' \
REASONING_MODEL='qwen3:32b' \
EMBEDDINGS_API_URL='https://enchanted.ngrok.pro/v1' \
EMBEDDINGS_MODEL='nomic-embed-text' \
IS_PROD_BUILD='false' \
NOTARY_API_KEY_ID=742ZY9FRN6 \
NOTARY_API_ISSUER=899fdbc2-cee9-4aea-b78b-850333a61f19 \
NOTARY_TEAM_ID=JDDZ55DT74 \
TELEGRAM_CHAT_SERVER=https://enchanted-proxy-telegram-dev.up.railway.app/query \
ENCHANTED_MCP_URL=https://enchanted-proxy-dev.up.railway.app/mcp \
POSTHOG_API_KEY=phc_z8xhkNCHHUClOYiQ79nLsMeY7rxbWqCpI8KQUmmcKd8 \
TTS_API_KEY=tinfoil-api-key \
TTS_MODEL=kokoro \
TTS_URL=https://audio-processing.model.tinfoil.sh/v1/ \
STT_API_KEY=tinfoil-api-key \
STT_MODEL="whisper-large-v3-turbo" \
STT_URL=https://audio-processing.model.tinfoil.sh/v1/ \
BUILD_CHANNEL=dev \
pnpm build-local:mac:dev
```

### Troubleshooting

- If you see a database (either SQLite or Weaviate) delete local directory `output` for testing.

- Application data on Mac is `~/Library/Application Support/enchanted`

- Logs data on Mac is `~/Library/Logs/enchanted`
