# Enchanted Twin

## Dev Requirements

- [Ollama](https://ollama.ai/) must be installed **and running** on your system
- Go
- pnpm
- Node.js
- Docker

## setup

### Ollama

You must have Ollama installed \*_and running_ on your system and running.

### Frontend

1. Copy `.env.sample` to `.env`
2. Set `RENDERER_VITE_API_URL` to the URL of your GraphQL API (if different from the default)
3. Navigate to the `app` directory
4. Install packages `pnpm install`
5. Run the app `cd app && pnpm dev`

### Backend

> ⚠️ Make sure ollama is running before running the backend

1. Navigate to `backend/golang`
1. Copy `.env.sample` to `.env` and update env variables
1. Install packages `make install`
1. Run the server `make run`

### GraphQL

On the backend side GraphQL resolvers (`schema.resolvers.go`) are code-generated from the schema `schema.graphqls`. Steps to update the schema

1. Propose schema changes in `schema.graphqls`.
1. Generate resolvers using `make gqlgen` in `backend/golang` directory.
1. This will generate additional code in `schema.resolvers.go`.

Frontend uses `schema.graphqls` as the source of truth to code generate queries/mutations/subscriptions using `pnpm codegen`.

## Release (build installer)

Navigate to the root.

For Mac M series use.

```sh
OPENAI_BASE_URL='https://enchanted.ngrok.pro/v1' \
COMPLETIONS_MODEL='mistral-small3.1' \
EMBEDDINGS_API_URL='https://enchanted.ngrok.pro/v1' \
EMBEDDINGS_MODEL='nomic-embed-text' \
IS_PROD_BUILD='true' \
OLLAMA_BASE_URL=https://enchanted.ngrok.pro \
NOTARY_API_KEY_ID=742ZY9FRN6 \
NOTARY_API_ISSUER=899fdbc2-cee9-4aea-b78b-850333a61f19 \
NOTARY_TEAM_ID=JDDZ55DT74 \
OLLAMA_BASE_URL=https://enchanted.ngrok.pro \
TELEGRAM_TOKEN=xxx \
make build-mac-silicon
```

To build for all architectures

```sh
OPENAI_BASE_URL='https://enchanted.ngrok.pro/v1' \
COMPLETIONS_MODEL='mistral-small3.1' \
EMBEDDINGS_API_URL='https://enchanted.ngrok.pro/v1/embeddings' \
EMBEDDINGS_MODEL='nomic-embed-text' \
IS_PROD_BUILD='true' \
OLLAMA_BASE_URL=https://enchanted.ngrok.pro \
make build-all
```
