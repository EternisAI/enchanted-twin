# TDLib Integration for Enchanted Twin

This document provides instructions for setting up and using the Telegram Database Library (TDLib) integration in the Enchanted Twin project.

## Prerequisites

1. Telegram API credentials (API ID and API Hash) from https://my.telegram.org/apps
2. TDLib installed on your system (see the installation guide)

## Environment Variables

Add the following to your `.env` file:

```
TELEGRAM_TDLIB_API_ID=your_api_id
TELEGRAM_TDLIB_API_HASH=your_api_hash
```

## Running the Application

```bash
# Set environment variables for TDLib
export CGO_CFLAGS="-I/usr/local/include"
export CGO_LDFLAGS="-Wl,-rpath,/usr/local/lib -L/usr/local/lib -ltdjson"

# Run the application
make run
```

## Authentication Process

When running the application for the first time:

1. You'll be prompted to enter your phone number (without the + sign)
2. You'll receive a verification code on your Telegram account
3. Enter the code when prompted
4. If you have two-factor authentication enabled, you'll be asked for your password

Authentication data is stored in the `./tdlib-db` directory, so you won't need to log in again in subsequent runs.

## Troubleshooting

### Rate Limiting

If you encounter a "429 Too Many Requests" error, you'll need to wait for the specified time before trying again. This is a Telegram API limitation.

### Authentication Failures

If authentication fails, delete the `./tdlib-db` directory and try again:

```bash
rm -rf ./tdlib-db
make run
```

### Missing Libraries

If you encounter errors about missing libraries, ensure TDLib is properly installed and the environment variables are set correctly.

## References

- [TDLib GitHub repository](https://github.com/tdlib/td)
- [Zelenin Go-TDLib wrapper](https://github.com/zelenin/go-tdlib)
