install:
	go install github.com/99designs/gqlgen
	brew install mockery
	brew install golangci-lint

run:
	go run cmd/server/*.go

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
