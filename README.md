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

## Release (build installer)

Navigate to the root.

For Mac M series use.

```sh
OPENAI_BASE_URL='https://enchanted.ngrok.pro/v1' \
COMPLETIONS_MODEL='mistral-small3.1' \
EMBEDDINGS_API_URL='https://enchanted.ngrok.pro/v1/embeddings' \
EMBEDDINGS_MODEL='nomic-embed-text' \
make build-mac-silicon
```

To build for all architectures

```sh
OPENAI_BASE_URL='https://enchanted.ngrok.pro/v1' \
COMPLETIONS_MODEL='mistral-small3.1' \
EMBEDDINGS_API_URL='https://enchanted.ngrok.pro/v1/embeddings' \
EMBEDDINGS_MODEL='nomic-embed-text' \
make build-all
```
