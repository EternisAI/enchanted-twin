install:
	go install github.com/99designs/gqlgen
	brew install mockery
	brew install golangci-lint

run:
	go run cmd/server/main.go

build:
	go build -o bin/enchanted-twin cmd/server/main.go

fresh-db:
	@echo "Removing all Weaviate and SQLite data to restart with fresh schema..."
	rm -rf ./output
	@echo "All data will be recreated on next server startup."

release:
	GOOS=darwin GOARCH=amd64 go build -o bin/enchanted-twin-darwin-amd64 cmd/server/main.go
	GOOS=darwin GOARCH=arm64 go build -o bin/enchanted-twin-darwin-arm64 cmd/server/main.go
	GOOS=linux GOARCH=amd64 go build -o bin/enchanted-twin-linux-amd64 cmd/server/main.go

lint:
	golangci-lint fmt
	golangci-lint run --fix

test:
	go test -short ./...

test-integration:
	go test -v ./pkg/dataprocessing/integration/... -timeout=90m

gqlgen:
	go run github.com/99designs/gqlgen generate

run-telegram-chat:
	go run cmd/telegram_chat_server/main.go

deadcode:
	@echo "=== Checking for transitively dead functions ==="
	@go run golang.org/x/tools/cmd/deadcode@latest ./...

sqlc-generate:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc generate

# CoreML Inference Server
build-coreml:
	@echo "Building CoreML inference server..."
	@mkdir -p bin/models
	@if [ -d "test_coreml" ]; then cp -r test_coreml/ bin/models/; fi
	CGO_ENABLED=1 go build -tags coreml -o bin/coreml-inference-server cmd/coreml_inference_server/main.go
	@echo "CoreML inference server built successfully"

run-coreml:
	@echo "Starting CoreML inference server..."
	go run -tags coreml cmd/coreml_inference_server/main.go

release-coreml:
	@echo "Building CoreML inference server for macOS..."
	@mkdir -p bin/models
	@if [ -d "test_coreml" ]; then cp -r test_coreml/ bin/models/; fi
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -tags coreml -o bin/coreml-inference-server-darwin-arm64 cmd/coreml_inference_server/main.go
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -tags coreml -o bin/coreml-inference-server-darwin-amd64 cmd/coreml_inference_server/main.go
	@echo "CoreML inference server release builds completed"

clean-coreml:
	@echo "Cleaning CoreML build artifacts..."
	rm -f bin/coreml-inference-server*
	rm -rf bin/models
	@echo "CoreML build artifacts cleaned"
