FROM python:3.11-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    postgresql \
    postgresql-client \
    libpq-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first for better caching
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

# Copy the scripts and source code
COPY scripts/ ./scripts/

# Directories will be mounted as volumes

# Copy GraphRAG package if it's a local package
# If GraphRAG is installed from PyPI in requirements.txt, this isn't needed
# COPY graphrag/ ./graphrag/

# Set environment variables
ENV PYTHONUNBUFFERED=1
ENV POSTGRES_USER=postgres
ENV POSTGRES_PASSWORD=postgres
ENV POSTGRES_DB=graphrag

# Entry point script
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

# Create volumes for persistent data
VOLUME ["/app/input_data"]
VOLUME ["/app/graphrag_root"]

ENTRYPOINT ["/app/entrypoint.sh"] 