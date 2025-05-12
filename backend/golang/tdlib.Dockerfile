FROM golang:1.20-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Set up working directory
WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/tdlib_service /app/cmd/tdlib_service
COPY pkg/telegram/tdlib_client.go /app/pkg/telegram/tdlib_client.go

# Build a version that doesn't require TDLib
RUN go build -tags notdlib -o /app/tdlib-service /app/cmd/tdlib_service

# Final stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl

# Set up working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/tdlib-service /app/tdlib-service

# Create a non-root user
RUN addgroup -g 1000 telegram && \
    adduser -u 1000 -G telegram -s /bin/sh -D telegram && \
    mkdir -p /tdlib/db /tdlib/files && \
    chown -R telegram:telegram /tdlib

USER telegram

# Create volumes for persistent data
VOLUME ["/tdlib/db", "/tdlib/files"]

# Set environment variables
ENV TELEGRAM_TDLIB_DB_DIR=/tdlib/db
ENV TELEGRAM_TDLIB_FILES_DIR=/tdlib/files

# Expose port for TDLib service
EXPOSE 9090

# Create a healthcheck
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:9090/health || exit 1

# Run the service
CMD ["/app/tdlib-service"]
