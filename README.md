# Enchanted

Enchanted is a personal AI assistant.

## Run

You can download signed binary from [Github releases](https://github.com/EternisAI/enchanted-twin/releases).

## Development

Requirements

- Go
- pnpm
- Node.js
- LLM ([Ollama](https://ollama.ai/), OpenAI, OpenRouter)

### Frontend

1. Navigate to the `app` directory
1. Install packages `pnpm install`
1. Run the app `cd app && pnpm dev`

### Backend

1. Navigate to `backend/golang`
1. Copy `.env.sample` to `.env` and update env variables
1. Run the server `make run`

### GraphQL

### Backend

On the backend side GraphQL resolvers (`schema.resolvers.go`) are code-generated from the schema `schema.graphqls`. Steps to update the schema

1. Propose schema changes in `schema.graphqls`.
1. Generate resolvers using `make gqlgen` in `backend/golang` directory.
1. This will generate additional code in `schema.resolvers.go`.

### Frontend

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
OLLAMA_BASE_URL=https://enchanted.ngrok.pro \
NOTARY_API_KEY_ID=742ZY9FRN6 \
NOTARY_API_ISSUER=899fdbc2-cee9-4aea-b78b-850333a61f19 \
NOTARY_TEAM_ID=JDDZ55DT74 \
OLLAMA_BASE_URL=https://enchanted.ngrok.pro \
TELEGRAM_TOKEN=xxx \
TELEGRAM_CHAT_SERVER=http://54.82.31.213:8080/query \
ENCHANTED_MCP_URL=https://08cace00a6a1a7bb1030eaf1bf3ba91a9759a91e-8080.dstack-prod6.phala.network/mcp \
POSTHOG_API_KEY=phc_z8xhkNCHHUClOYiQ79nLsMeY7rxbWqCpI8KQUmmcKd8 \
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
OLLAMA_BASE_URL=https://enchanted.ngrok.pro \
NOTARY_API_KEY_ID=742ZY9FRN6 \
NOTARY_API_ISSUER=899fdbc2-cee9-4aea-b78b-850333a61f19 \
NOTARY_TEAM_ID=JDDZ55DT74 \
OLLAMA_BASE_URL=https://enchanted.ngrok.pro \
TELEGRAM_TOKEN=xxx \
TELEGRAM_CHAT_SERVER=http://54.82.31.213:8080/query \
ENCHANTED_MCP_URL=https://08cace00a6a1a7bb1030eaf1bf3ba91a9759a91e-8080.dstack-prod6.phala.network/mcp \
POSTHOG_API_KEY=phc_z8xhkNCHHUClOYiQ79nLsMeY7rxbWqCpI8KQUmmcKd8 \
make build-all
```

Local build

```sh
COMPLETIONS_API_URL='https://enchanted.ngrok.pro/v1' \
COMPLETIONS_MODEL='mistral-small3.1' \
REASONING_MODEL='qwen3:32b' \
EMBEDDINGS_API_URL='https://enchanted.ngrok.pro/v1' \
EMBEDDINGS_MODEL='nomic-embed-text' \
IS_PROD_BUILD='true' \
OLLAMA_BASE_URL=https://enchanted.ngrok.pro \
NOTARY_API_KEY_ID=742ZY9FRN6 \
NOTARY_API_ISSUER=899fdbc2-cee9-4aea-b78b-850333a61f19 \
NOTARY_TEAM_ID=JDDZ55DT74 \
OLLAMA_BASE_URL=https://enchanted.ngrok.pro \
TELEGRAM_TOKEN=xxx \
TELEGRAM_CHAT_SERVER=http://54.82.31.213:8080/query \
ENCHANTED_MCP_URL=https://08cace00a6a1a7bb1030eaf1bf3ba91a9759a91e-8080.dstack-prod6.phala.network/mcp \
POSTHOG_API_KEY=phc_z8xhkNCHHUClOYiQ79nLsMeY7rxbWqCpI8KQUmmcKd8 \
pnpm build-local:mac
```

### Troubleshooting

- If you see a database (either SQLite or Weaviate) delete local directory `output` for testing.

- Application data on Mac is `~/Library/Application Support/enchanted`

- Logs data on Mac is `~/Library/Logs/enchanted`
