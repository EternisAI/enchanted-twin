#!/bin/bash
set -e

# Define default environment variables and allow overrides
: "${GRAPHRAG_ROOT:="/app/graphrag_root"}"
: "${INPUT_DATA_DIR:="/app/input_data"}"
: "${DO_INIT:="auto"}"           # auto, true, false
: "${DO_INDEX:="false"}"          # true, false - includes data processing
: "${POSTGRES_USER:="postgres"}"
: "${POSTGRES_PASSWORD:="postgres"}"
: "${POSTGRES_DB:="graphrag"}"

echo "GraphRAG root directory: $GRAPHRAG_ROOT"
ls "$GRAPHRAG_ROOT"
echo "Input data directory: $INPUT_DATA_DIR"
ls "$INPUT_DATA_DIR"

# because GraphRAG has to be super special
export GRAPHRAG_API_KEY="$OPENAI_API_KEY"

# Directories are mounted as volumes, no need to create them

# Initialize GraphRAG if needed
if [ "$DO_INIT" = "true" ] || ([ "$DO_INIT" = "auto" ] && [ ! -f "$GRAPHRAG_ROOT/.env" ]); then
    echo "Initializing GraphRAG..."
    graphrag init --root "$GRAPHRAG_ROOT"
    echo "GraphRAG initialization complete."

	# Update .env file with API key
	echo "Updating GraphRAG configuration..."
	if [ -n "$OPENAI_API_KEY" ]; then
		echo "GRAPHRAG_API_KEY=$OPENAI_API_KEY" > "$GRAPHRAG_ROOT/.env"
		echo "Added API key to GraphRAG configuration"
	else
		echo "WARNING: OPENAI_API_KEY is not set, GraphRAG may not work properly"
	fi
	
	# Update settings.yaml with correct model and file type
	SETTINGS_FILE="$GRAPHRAG_ROOT/settings.yaml"
	if [ -f "$SETTINGS_FILE" ]; then
		echo "Updating settings.yaml..."
		
		# Update model to gpt-4o-mini -- can't handle gpt-4.1-mini due to old libs
		sed -i 's/model: gpt.*$/model: gpt-4o-mini/g' "$SETTINGS_FILE"
		
		# Update file_type to csv
		sed -i 's/file_type: .*$/file_type: jsonl/g' "$SETTINGS_FILE"

		cat "$SETTINGS_FILE"
		echo
		echo "------------------------"
		echo
	else
		echo "WARNING: settings.yaml not found at $SETTINGS_FILE"
	fi


	# Initialize PostgreSQL
	echo "Initializing PostgreSQL..."
	service postgresql start
	su - postgres -c "psql -c \"ALTER USER postgres WITH PASSWORD '$POSTGRES_PASSWORD';\""
	su - postgres -c "psql -c \"CREATE DATABASE $POSTGRES_DB;\"" || echo "Database may already exist"
else
    echo "Skipping GraphRAG initialization (DO_INIT=$DO_INIT)"
fi

# Run indexing and data processing if enabled
if [ "$DO_INDEX" = "true" ]; then
    echo "Processing and indexing data..."
    
    # Process data files before indexing
    # Process slack data if it exists
    if [ -f "$INPUT_DATA_DIR/slack.csv" ]; then
        echo "Processing Slack data..."
        python ./scripts/prepare_slack.py
    fi

    # Process telegram data if it exists
    if [ -f "$INPUT_DATA_DIR/telegram.csv" ]; then
        echo "Processing Telegram data..."
        python ./scripts/prepare_telegram.py
    fi
    
    # Run the indexing
    echo "Indexing GraphRAG data..."
    graphrag index --root "$GRAPHRAG_ROOT"
    echo "GraphRAG indexing complete."

	echo "simplemem.py indexing..."
	python ./scripts/simplemem.py --db-user="$POSTGRES_USER" --db-password="$POSTGRES_PASSWORD" --db-name="$POSTGRES_DB" --graphrag-folder "$GRAPHRAG_ROOT"
	echo "simplemem.py indexing complete"
fi

# Run memory query if we're passed in parameters
if [ $# -gt 0 ]; then
    echo "Calling simplemem.py with parameters:" "$@"
    python ./scripts/simplemem.py --db-user="$POSTGRES_USER" --db-password="$POSTGRES_PASSWORD" --db-name="$POSTGRES_DB" "$@"
else
    echo "No parameters passed, skipping simplemem.py execution"
fi
