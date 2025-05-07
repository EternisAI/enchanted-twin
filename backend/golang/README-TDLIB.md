# TDLib Integration for Telegram

This document explains how to set up and use the TDLib (Telegram Database Library) integration.

## Prerequisites

1. Install TDLib according to [the official instructions](https://tdlib.github.io/td/build.html)
2. Register a Telegram application at https://my.telegram.org/apps to get API ID and hash

## Configuration

Set the following environment variables:

```
TELEGRAM_TDLIB_API_ID=your_api_id
TELEGRAM_TDLIB_API_HASH=your_api_hash
```

## Authentication

When you start the application with valid API credentials, you'll be prompted to authenticate:

1. For the first run, you'll need to input your phone number when prompted
2. Enter the authentication code sent to your Telegram account
3. If you have two-factor authentication enabled, you'll be asked for your password

## Usage

Once authenticated, the TDLib client will be available within the application. Authentication data is stored in the `./tdlib-db` directory, so you won't need to log in again in subsequent runs.

## Troubleshooting

- Make sure TDLib is properly installed on your system
- Check that your API ID and hash are correctly configured
- If you encounter authentication issues, delete the `./tdlib-db` directory and try again

## References

- [TDLib GitHub repository](https://github.com/tdlib/td)
- [Zelenin Go-TDLib wrapper](https://github.com/zelenin/go-tdlib)
- [Telegram API documentation](https://core.telegram.org/api) 