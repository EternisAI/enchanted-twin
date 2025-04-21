# Enchanted Twin

## Dev Requirements

- [Ollama](https://ollama.ai/) must be installed **and running** on your system
- Go
- pnpm
- Node.js
- Docker

## setup

### ollama

You must have Ollama installed \*_and running_ on your system and running.

### frontend

1. Copy `.env.sample` to `.env`
2. Set `RENDERER_VITE_API_URL` to the URL of your GraphQL API (if different from the default)
3. Navigate to the `app` directory
4. Install packages `pnpm install`
5. Run the app `cd app && pnpm dev`

### backend

1. Navigate to `backend/golang`
2. Install packages `make install`
3. Run the server `make run`

## build & release

Navigate to the root.

For Mac M series use.

```sh
make build-mac-silicon
```

To build for all architectures

```sh
make build-all
```
