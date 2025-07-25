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
	find . -name "*.db" -not -path "./cmd/memory-processor-test/*" -not -path "./weaviate-test-memory/*" -delete 2>/dev/null || true
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
	go test -short ./...

test-integration:
	go test -v ./pkg/dataprocessing/integration/... -timeout=90m

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
	go run go.uber.org/nilaway/cmd/nilaway@latest -json -pretty-print=false ./pkg/telegram/...