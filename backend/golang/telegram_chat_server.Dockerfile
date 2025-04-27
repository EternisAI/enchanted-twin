FROM golang:1.24.2 AS builder
WORKDIR /app

# Copy module files from the correct location in the build context
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the entire project context
COPY . .

# Build the application with CGO enabled for SQLite support
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /app/server ./cmd/telegram_chat_server

# --- Final Stage ---
FROM debian:bookworm-slim
WORKDIR /app

# Install SQLite and other dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    sqlite3 \
    && rm -rf /var/lib/apt/lists/*

# Copy only the built binary
COPY --from=builder /app/server .

# Ensure the binary is executable
RUN chmod +x /app/server

# Create data directory with proper permissions
RUN mkdir -p /app/data

# Create a non-root user
RUN useradd -r -u 1000 -m appuser && chown -R appuser:appuser /app/data
RUN chown appuser:appuser /app
USER appuser

CMD ["/app/server"]