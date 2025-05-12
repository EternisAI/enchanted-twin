# TDLib Docker Integration for Enchanted Twin

This document provides instructions for setting up and using the containerized TDLib service in the Enchanted Twin project.

## Overview

Instead of installing TDLib locally, this setup uses Docker to provide a containerized TDLib service with phone authentication. This approach has several advantages:

1. No need to install TDLib and its dependencies on the host machine
2. Consistent environment across different platforms (macOS, Linux, Windows)
3. Easier deployment and scaling
4. Isolation of TDLib interactions from the main application
5. Persistent storage for authentication data
6. Memory-efficient build process using conditional compilation

## Implementation Details

The TDLib service is implemented with two different build variants:

1. **Full TDLib Implementation** - Used when building without special tags
2. **Mock TDLib Implementation** - Used when building with `-tags notdlib`

The Docker setup uses the mock implementation by default to avoid memory-intensive compilation on resource-constrained systems like macOS. This approach provides the same API interface while significantly reducing build resource requirements.

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

The TDLib service requires phone authentication. The process works as follows:

1. Start the TDLib service using Docker Compose
2. Check the authentication state using the API endpoint
3. When prompted, provide your phone number
4. Receive the authentication code via Telegram
5. Provide the authentication code to the service
6. Once authenticated, the service will maintain the session

### Authentication API Endpoints

- `GET /api/authState` - Get the current authentication state
- `POST /api/setPhoneNumber` - Provide your phone number for authentication
- `POST /api/setAuthCode` - Provide the authentication code received via Telegram

### Authentication States

The TDLib service can be in one of the following authentication states:

- `waiting_for_parameters` - Initial state, waiting for TDLib parameters
- `waiting_for_phone_number` - Waiting for the user to provide a phone number
- `waiting_for_code` - Waiting for the authentication code
- `waiting_for_password` - Waiting for the account password (if 2FA is enabled)
- `authorized` - Successfully authenticated
- `ready` - TDLib client is initialized and ready to use

### Example Authentication Flow

1. Check the authentication state:
   ```bash
   curl http://localhost:9090/api/authState
   ```

2. If the state is `waiting_for_phone_number`, provide your phone number:
   ```bash
   curl -X POST -H "Content-Type: application/json" -d '{"phone_number":"+1234567890"}' http://localhost:9090/api/setPhoneNumber
   ```

3. Wait for the authentication code to be sent to your Telegram account

4. Provide the authentication code:
   ```bash
   curl -X POST -H "Content-Type: application/json" -d '{"code":"12345"}' http://localhost:9090/api/setAuthCode
   ```

5. Check the authentication state again to confirm successful authentication:
   ```bash
   curl http://localhost:9090/api/authState
   ```

## API Endpoints

The TDLib service provides the following endpoints:

- `GET /health` - Health check endpoint
- `GET /api/authState` - Get the current authentication state
- `POST /api/setPhoneNumber` - Provide your phone number for authentication
- `POST /api/setAuthCode` - Provide the authentication code received via Telegram
- `GET /api/getMe` - Get information about the authenticated user
- `GET /api/getChats` - Get list of chats
- `POST /api/sendMessage` - Send a message to a chat

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

If you encounter authentication issues:

1. Check the authentication state using the `/api/authState` endpoint
2. Ensure you're providing the correct phone number in international format (e.g., +1234567890)
3. Make sure you're entering the correct authentication code
4. Check the logs for any error messages

### Connection Issues

If the main application cannot connect to the TDLib service, ensure:

1. The TDLib service is running (`docker ps`)
2. The `TELEGRAM_TDLIB_SERVICE_URL` environment variable is set correctly
3. There are no network issues between the main application and the TDLib service

## References

- [TDLib Documentation](https://core.telegram.org/tdlib)
- [Telegram API Credentials](https://my.telegram.org/apps)
