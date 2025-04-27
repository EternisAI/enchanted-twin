FROM golang:1.24.2 AS builder
WORKDIR /app

# Copy module files from the correct location in the build context
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the entire project context
COPY . .

# Build the application, specifying the correct package path
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/telegram_chat_server

# --- Final Stage ---
FROM alpine:latest
WORKDIR /app
# Copy only the built binary
COPY --from=builder /app/server .

# Ensure the binary is executable
RUN chmod +x /app/server

RUN adduser -D -u 1000 appuser
USER appuser

CMD ["/app/server"]

