install:
	go install github.com/99designs/gqlgen
	brew install mockery
	brew install golangci-lint

run:
	go run cmd/server/main.go

build:
	go build -o bin/enchanted-twin cmd/server/main.go

fresh-db:
	@echo "Removing all PostgreSQL and SQLite data to restart with fresh schema..."
	rm -rf ./output
	rm -rf ./postgres-data 2>/dev/null || true
	find . -name "*.db" -delete 2>/dev/null || true
	find . -name "*.db-wal" -delete 2>/dev/null || true  
	find . -name "*.db-shm" -delete 2>/dev/null || true
	@echo "All data will be recreated on next server startup."

release:
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o bin/enchanted-twin-darwin-amd64 cmd/server/main.go
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o bin/enchanted-twin-darwin-arm64 cmd/server/main.go
	# CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o bin/enchanted-twin-linux-amd64 cmd/server/main.go

lint:
	golangci-lint fmt
	golangci-lint run --fix

test:
ifeq ($(CI),true)
	go test -short ./...
else ifeq ($(GITHUB_ACTIONS),true)
	go test -short ./...
else
	go test -short -tags=large ./...
endif

test-containers:
	@echo "Running tests with testcontainers..."
	USE_TESTCONTAINERS=true go test -v -tags=large ./pkg/agent/memory/evolvingmemory/... -timeout=10m

test-backend-comparison:
	@echo "Running backend comparison tests with containers..."
	USE_TESTCONTAINERS=true ENABLE_BACKEND_COMPARISON_TESTS=true go test -v -tags=large ./pkg/agent/memory/evolvingmemory/... -run=".*Comparison.*" -timeout=15m

test-integration:
	go test -v ./pkg/dataprocessing/integration/... -timeout=90m

test-integration-containers:
	@echo "Running integration tests with testcontainers..."
	USE_TESTCONTAINERS=true go test -v ./pkg/dataprocessing/integration/... -timeout=90m

gqlgen:
	go run github.com/99designs/gqlgen generate

run-telegram-chat:
	go run cmd/telegram_chat_server/main.go

deadcode:
	@echo "=== Checking for transitively dead functions ==="
	@go run golang.org/x/tools/cmd/deadcode@latest -test ./...

sqlc-generate:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc generate

nilcheck:
	go run go.uber.org/nilaway/cmd/nilaway@latest -json -v -pretty-print=false ./pkg/... 
	#replace './pkg/...' with the path to the package you want to check for faster results

fetch-deps:
	@echo "Fetching runtime dependencies..."
	@command -v jq >/dev/null 2>&1 || { echo "jq is required but not installed. Install with: brew install jq"; exit 1; }
	@ROOT_DIR="$(shell cd ../.. && pwd)" && \
	ENV_FILE="$$ROOT_DIR/.env" && \
	if [ -f "$$ENV_FILE" ]; then \
		echo "Loading environment from $$ENV_FILE"; \
		set -a && . "$$ENV_FILE" && set +a; \
	fi && \
	DEPS_CONFIG="$$ROOT_DIR/runtime-dependencies.json" && \
	if [ ! -f "$$DEPS_CONFIG" ]; then \
		echo "Error: runtime-dependencies.json not found at $$DEPS_CONFIG"; \
		exit 1; \
	fi && \
	DEPENDENCIES_DIR="$${APP_DATA_PATH:-./output}" && \
	echo "Installing dependencies to: $$DEPENDENCIES_DIR" && \
	mkdir -p "$$DEPENDENCIES_DIR" && \
	\
	for dep in $$(jq -r '.dependencies | keys[]' "$$DEPS_CONFIG"); do \
		DEP_UPPER=$$(echo "$$dep" | tr '[:lower:]' '[:upper:]'); \
		ENABLED_VAR="$${DEP_UPPER}_BACKEND_DOWNLOAD_ENABLED"; \
		ENV_ENABLED=$$(eval "echo \$$$$ENABLED_VAR"); \
		JSON_ENABLED=$$(jq -r ".dependencies.$$dep.backend_download_enabled" "$$DEPS_CONFIG"); \
		BACKEND_DOWNLOAD_ENABLED="$${ENV_ENABLED:-$$JSON_ENABLED}"; \
		if [ "$$BACKEND_DOWNLOAD_ENABLED" = "false" ]; then \
			echo "⊘ Skipping $$dep (backend download disabled)"; \
			continue; \
		fi; \
		\
		DEP_DIR=$$(jq -r ".dependencies.$$dep.dir" "$$DEPS_CONFIG" | sed "s|{DEPENDENCIES_DIR}|$$DEPENDENCIES_DIR|g"); \
		\
		echo "→ Checking $$dep..."; \
		ALREADY_DOWNLOADED="true"; \
		VALIDATION_FILES_TYPE=$$(jq -r "type" <<< "$$(jq ".dependencies.$$dep.validation_files" "$$DEPS_CONFIG")"); \
		if [ "$$VALIDATION_FILES_TYPE" = "object" ]; then \
			PLATFORM=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
			ARCH=$$(uname -m); \
			if [ "$$PLATFORM" = "darwin" ] && [ "$$ARCH" = "arm64" ]; then \
				PLATFORM_KEY="darwin-arm64"; \
			else \
				PLATFORM_KEY="linux-x64"; \
			fi; \
			for validation_file in $$(jq -r ".dependencies.$$dep.validation_files.\"$$PLATFORM_KEY\"[]?" "$$DEPS_CONFIG"); do \
				if [ ! -e "$$DEP_DIR/$$validation_file" ]; then \
					ALREADY_DOWNLOADED="false"; \
					break; \
				fi; \
			done; \
		else \
			for validation_file in $$(jq -r ".dependencies.$$dep.validation_files[]?" "$$DEPS_CONFIG"); do \
				if [ ! -e "$$DEP_DIR/$$validation_file" ]; then \
					ALREADY_DOWNLOADED="false"; \
					break; \
				fi; \
			done; \
		fi; \
		\
		if [ "$$ALREADY_DOWNLOADED" = "true" ]; then \
			echo "  ⚠️  Already downloaded, skipping"; \
			continue; \
		fi; \
		\
		echo "  Downloading $$dep..."; \
		mkdir -p "$$DEP_DIR"; \
		TYPE=$$(jq -r ".dependencies.$$dep.type" "$$DEPS_CONFIG"); \
		\
		case "$$TYPE" in \
			"individual_files") \
				BASE_URL=$$(jq -r ".dependencies.$$dep.url" "$$DEPS_CONFIG"); \
				for category in binaries libraries dataFiles; do \
					for file in $$(jq -r ".dependencies.$$dep.files.$$category[]?" "$$DEPS_CONFIG"); do \
						echo "    $$file"; \
						mkdir -p "$$(dirname "$$DEP_DIR/$$file")"; \
						curl -fsSL "$$BASE_URL/$$file" -o "$$DEP_DIR/$$file" || { echo "Failed to download $$file"; exit 1; }; \
					done; \
				done; \
				;; \
			"zip") \
				URL=$$(jq -r ".dependencies.$$dep.url" "$$DEPS_CONFIG"); \
				curl -fsSL "$$URL" -o "$$DEP_DIR/temp.zip"; \
				cd "$$DEP_DIR" && unzip -q temp.zip && rm temp.zip; \
				;; \
			"tar.gz") \
				PLATFORM_KEY=$$(jq -r ".dependencies.$$dep.platform_url_key // false" "$$DEPS_CONFIG"); \
				if [ "$$PLATFORM_KEY" = "true" ]; then \
					PLATFORM=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
					ARCH=$$(uname -m); \
					if [ "$$PLATFORM" = "darwin" ] && [ "$$ARCH" = "arm64" ]; then \
						URL=$$(jq -r ".dependencies.$$dep.url.\"darwin-arm64\"" "$$DEPS_CONFIG"); \
					else \
						URL=$$(jq -r ".dependencies.$$dep.url.\"linux-x64\"" "$$DEPS_CONFIG"); \
					fi; \
				else \
					URL=$$(jq -r ".dependencies.$$dep.url" "$$DEPS_CONFIG"); \
				fi; \
				curl -fsSL "$$URL" -o "$$DEP_DIR/temp.tgz"; \
				cd "$$DEP_DIR" && tar -xzf temp.tgz && rm temp.tgz; \
				;; \
			"curl_script") \
				SCRIPT=$$(jq -r ".dependencies.$$dep.install_script" "$$DEPS_CONFIG"); \
				eval "$$SCRIPT"; \
				if command -v uv >/dev/null 2>&1; then \
					cp "$$(which uv)" "$$DEP_DIR/uv" 2>/dev/null || true; \
				fi; \
				;; \
		esac; \
		\
		if jq -e ".dependencies.$$dep.post_download.chmod" "$$DEPS_CONFIG" >/dev/null; then \
			MODE=$$(jq -r ".dependencies.$$dep.post_download.chmod.mode" "$$DEPS_CONFIG"); \
			for file in $$(jq -r ".dependencies.$$dep.post_download.chmod.files[]" "$$DEPS_CONFIG"); do \
				chmod "$$MODE" "$$DEP_DIR/$$file" 2>/dev/null || true; \
			done; \
		fi; \
	done && \
	\
	echo "✓ All dependencies processed successfully!" && \
	echo "Dependencies installed to: $$DEPENDENCIES_DIR"

clean-deps:
	@ROOT_DIR="$(shell cd ../.. && pwd)" && \
	ENV_FILE="$$ROOT_DIR/.env" && \
	if [ -f "$$ENV_FILE" ]; then \
		set -a && . "$$ENV_FILE" && set +a; \
	fi && \
	DEPENDENCIES_DIR="$${APP_DATA_PATH:-./output}" && \
	echo "This will remove all downloaded dependencies from $$DEPENDENCIES_DIR" && \
	read -p "Are you sure? (y/N): " confirm && [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ] || { echo "Cancelled"; exit 1; } && \
	rm -rf "$$DEPENDENCIES_DIR" && \
	echo "✓ Dependencies cleaned from $$DEPENDENCIES_DIR"