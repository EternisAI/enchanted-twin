FROM ubuntu:22.04 as builder

# Install dependencies for building TDLib
RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    gperf \
    git \
    zlib1g-dev \
    libssl-dev \
    pkg-config \
    golang \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Clone and build TDLib
WORKDIR /src
RUN git clone --depth 1 --branch v1.8.0 https://github.com/tdlib/td.git && \
    cd td && \
    mkdir build && \
    cd build && \
    cmake -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX:PATH=/usr/local .. && \
    cmake --build . --target install -j $(nproc) --config Release

# Second stage for the final image
FROM ubuntu:22.04

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    libssl3 \
    zlib1g \
    golang \
    && rm -rf /var/lib/apt/lists/*

# Copy TDLib from builder
COPY --from=builder /usr/local/lib/libtd* /usr/local/lib/
COPY --from=builder /usr/local/include/td /usr/local/include/td
RUN ldconfig

# Set up working directory
WORKDIR /app

# Copy source code
COPY cmd/tdlib_service /app/cmd/tdlib_service
COPY pkg/telegram/tdlib_client.go /app/pkg/telegram/tdlib_client.go
COPY go.mod go.sum /app/

# Build the TDLib service
RUN go build -o /app/tdlib-service /app/cmd/tdlib_service

# Create a non-root user
RUN groupadd -g 1000 telegram && \
    useradd -u 1000 -g telegram -s /bin/bash -m telegram && \
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
