FROM golang:1.24.2 AS builder

# Install TDLib dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    gperf \
    git \
    zlib1g-dev \
    libssl-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

# Clone and build TDLib
WORKDIR /src
RUN git clone https://github.com/tdlib/td.git && \
    cd td && \
    mkdir build && \
    cd build && \
    cmake -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX:PATH=/usr/local .. && \
    cmake --build . --target install -j $(nproc)

# Set up Go environment
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the tdlib service
RUN CGO_ENABLED=1 \
    CGO_CFLAGS="-I/usr/local/include" \
    CGO_LDFLAGS="-Wl,-rpath,/usr/local/lib -L/usr/local/lib -ltdjson" \
    go build -o /app/tdlib-service ./cmd/tdlib_service

# Create a smaller runtime image
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    libssl3 \
    zlib1g \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy TDLib from builder
COPY --from=builder /usr/local/lib/libtd* /usr/local/lib/
COPY --from=builder /usr/local/include/td /usr/local/include/td

# Copy the compiled binary
COPY --from=builder /app/tdlib-service /usr/local/bin/tdlib-service

# Update the dynamic linker run-time bindings
RUN ldconfig

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
