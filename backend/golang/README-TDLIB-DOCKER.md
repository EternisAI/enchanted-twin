# Telegram API Docker Integration for Enchanted Twin

This document provides instructions for setting up and using the containerized Telegram API service in the Enchanted Twin project.

## Overview

Instead of installing TDLib locally, this setup uses Docker to provide a lightweight REST API service for interacting with the Telegram API. This approach has several advantages:

1. No need to install TDLib and its dependencies on the host machine
2. Minimal resource requirements (works well on macOS with limited memory)
3. Easier deployment and scaling
4. Isolation of Telegram API interactions from the main application

## Prerequisites

1. Docker and Docker Compose installed on your system
2. Telegram Bot Token from https://t.me/BotFather

## Environment Variables

Add the following to your `.env` file:

```
TELEGRAM_TOKEN=your_bot_token
TELEGRAM_TDLIB_SERVICE_URL=http://localhost:9090
```

## Running the Telegram API Service

```bash
# Start the Telegram API service
cd backend/golang
docker-compose -f docker-compose.tdlib.yml up -d

# Check the logs
docker-compose -f docker-compose.tdlib.yml logs -f
```

## API Endpoints

The Telegram API service provides the following endpoints:

- `GET /health` - Health check endpoint
- `GET /api/getMe` - Get information about the bot
- `GET /api/getChats` - Get list of chats
- `POST /api/sendMessage` - Send a message to a chat

## Stopping the Service

```bash
# Stop the Telegram API service
docker-compose -f docker-compose.tdlib.yml down
```

## Troubleshooting

### Service Not Starting

If the service fails to start, check the logs:

```bash
docker-compose -f docker-compose.tdlib.yml logs telegram-api
```

### Connection Issues

If the main application cannot connect to the Telegram API service, ensure:

1. The Telegram API service is running (`docker ps`)
2. The `TELEGRAM_TDLIB_SERVICE_URL` environment variable is set correctly
3. There are no network issues between the main application and the Telegram API service

## References

- [Telegram Bot API](https://core.telegram.org/bots/api)
