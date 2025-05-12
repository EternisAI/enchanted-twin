FROM golang:1.21-bookworm AS builder

# Install TDLib from official repository
RUN apt-get update && apt-get install -y \
    wget \
    gnupg \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Add TDLib repository and install pre-built package
RUN wget -qO- https://td.telegram.org/debian/td-apt-key.asc | gpg --dearmor > /usr/share/keyrings/td-apt-key.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/td-apt-key.gpg] https://td.telegram.org/debian bookworm main" > /etc/apt/sources.list.d/td.list && \
    apt-get update && \
    apt-get install -y libtdjson1 libtdjson-dev && \
    rm -rf /var/lib/apt/lists/*

# Set up Go environment
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the tdlib service
RUN CGO_ENABLED=1 \
    CGO_CFLAGS="-I/usr/include" \
    CGO_LDFLAGS="-Wl,-rpath,/usr/lib -L/usr/lib -ltdjson" \
    go build -o /app/tdlib-service ./cmd/tdlib_service

# Create a smaller runtime image
FROM debian:bookworm-slim

# Install TDLib from official repository
RUN apt-get update && apt-get install -y \
    wget \
    gnupg \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Add TDLib repository and install pre-built package
RUN wget -qO- https://td.telegram.org/debian/td-apt-key.asc | gpg --dearmor > /usr/share/keyrings/td-apt-key.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/td-apt-key.gpg] https://td.telegram.org/debian bookworm main" > /etc/apt/sources.list.d/td.list && \
    apt-get update && \
    apt-get install -y libtdjson1 && \
    rm -rf /var/lib/apt/lists/*

# Copy the compiled binary
COPY --from=builder /app/tdlib-service /usr/local/bin/tdlib-service

# Create directories for TDLib data
RUN mkdir -p /tdlib/db /tdlib/files
VOLUME ["/tdlib/db", "/tdlib/files"]

# Create a non-root user
RUN groupadd -g 999 tdlib && \
    useradd -r -u 1000 -g tdlib tdlib && \
    chown -R tdlib:tdlib /tdlib
USER tdlib

# Set environment variables
ENV TELEGRAM_TDLIB_DB_DIR=/tdlib/db
ENV TELEGRAM_TDLIB_FILES_DIR=/tdlib/files

# Expose port for tdlib service
EXPOSE 9090

# Run the service
CMD ["/usr/local/bin/tdlib-service"]
