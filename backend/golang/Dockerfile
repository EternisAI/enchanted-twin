FROM golang:1.24.2 AS builder
WORKDIR /app

# Copy module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the entire project
COPY . .

# Build the application with CGO enabled for SQLite support
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /app/server ./cmd/server

# --- Final Stage ---
FROM debian:bookworm-slim
WORKDIR /app

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    sqlite3 \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Copy the built binary
COPY --from=builder /app/server .

# Ensure the binary is executable
RUN chmod +x /app/server

# Create directories with proper permissions
RUN mkdir -p /app/output /app/output_test /app/data && \
    groupadd -g 999 appuser && \
    useradd -r -u 1000 -g 999 -m appuser && \
    chown -R appuser:appuser /app

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:${GRAPHQL_PORT:-44999}/query -X POST \
        -H "Content-Type: application/json" \
        -d '{"query": "{ __typename }"}' || exit 1

USER appuser

# Default environment variables
ENV GRAPHQL_PORT=44999
ENV WEAVIATE_PORT=51415
ENV DB_PATH=./output/sqlite/store.db
ENV APP_DATA_PATH=./output

EXPOSE 44999 51415

CMD ["/app/server"]