# TDLib Docker Integration for Enchanted Twin

This document provides instructions for setting up and using the containerized Telegram Database Library (TDLib) integration in the Enchanted Twin project.

## Overview

Instead of installing TDLib locally, this setup uses Docker to containerize TDLib and provide a REST API service for interacting with it. This approach has several advantages:

1. No need to install TDLib and its dependencies on the host machine
2. Consistent environment across different development and production setups
3. Easier deployment and scaling
4. Isolation of TDLib from the main application

## Prerequisites

1. Docker and Docker Compose installed on your system
2. Telegram API credentials (API ID and API Hash) from https://my.telegram.org/apps

## Environment Variables

Add the following to your `.env` file:

```
TELEGRAM_TDLIB_API_ID=your_api_id
TELEGRAM_TDLIB_API_HASH=your_api_hash
TELEGRAM_TDLIB_SERVICE_URL=http://localhost:9090
```

## Running the TDLib Service

```bash
# Start the TDLib service
cd backend/golang
docker-compose -f docker-compose.tdlib.yml up -d

# Check the logs
docker-compose -f docker-compose.tdlib.yml logs -f
```

## Authentication Process

When running the TDLib service for the first time, you'll need to authenticate with Telegram:

1. Check the logs of the tdlib-service container for authentication prompts
2. Follow the instructions to enter your phone number and verification code
3. If you have two-factor authentication enabled, you'll be asked for your password

Authentication data is stored in the Docker volume, so you won't need to log in again in subsequent runs.

## Stopping the Service

```bash
# Stop the TDLib service
docker-compose -f docker-compose.tdlib.yml down
```

## Troubleshooting

### Service Not Starting

If the service fails to start, check the logs:

```bash
docker-compose -f docker-compose.tdlib.yml logs tdlib-service
```

### Authentication Issues

If you encounter authentication issues, you can reset the authentication data by removing the Docker volumes:

```bash
docker-compose -f docker-compose.tdlib.yml down -v
docker-compose -f docker-compose.tdlib.yml up -d
```

### Connection Issues

If the main application cannot connect to the TDLib service, ensure:

1. The TDLib service is running (`docker ps`)
2. The `TELEGRAM_TDLIB_SERVICE_URL` environment variable is set correctly
3. There are no network issues between the main application and the TDLib service

## References

- [TDLib GitHub repository](https://github.com/tdlib/td)
- [Zelenin Go-TDLib wrapper](https://github.com/zelenin/go-tdlib)
