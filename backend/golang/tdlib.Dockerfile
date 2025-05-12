FROM debian:bookworm-slim

# Install TDLib and dependencies
RUN apt-get update && apt-get install -y \
    wget \
    gnupg \
    ca-certificates \
    libssl3 \
    zlib1g \
    && rm -rf /var/lib/apt/lists/*

# Add TDLib repository and install pre-built package
RUN wget -qO- https://td.telegram.org/debian/td-apt-key.asc | gpg --dearmor > /usr/share/keyrings/td-apt-key.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/td-apt-key.gpg] https://td.telegram.org/debian bookworm main" > /etc/apt/sources.list.d/td.list && \
    apt-get update && \
    apt-get install -y libtdjson1 libtdjson-dev && \
    rm -rf /var/lib/apt/lists/*

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

# Create a healthcheck
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD [ "test", "-d", "/tdlib/db" ]

# Keep container running
CMD ["tail", "-f", "/dev/null"]
