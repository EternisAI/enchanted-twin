FROM golang:1.24-alpine

# Install dependencies
RUN apk add --no-cache ca-certificates tzdata

# Set up working directory
WORKDIR /app

# Copy source code
COPY cmd/telegram_api_service /app/cmd/telegram_api_service
COPY pkg/telegram/api_client.go /app/pkg/telegram/api_client.go
COPY go.mod go.sum /app/

# Build the telegram API service
RUN go build -o /app/telegram-api-service /app/cmd/telegram_api_service

# Create a non-root user
RUN addgroup -g 1000 telegram && \
    adduser -u 1000 -G telegram -s /bin/sh -D telegram && \
    mkdir -p /data && \
    chown -R telegram:telegram /data
USER telegram

# Create volume for persistent data
VOLUME ["/data"]

# Set environment variables
ENV TELEGRAM_DATA_DIR=/data

# Expose port for telegram API service
EXPOSE 9090

# Create a healthcheck
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:9090/health || exit 1

# Run the service
CMD ["/app/telegram-api-service"]
