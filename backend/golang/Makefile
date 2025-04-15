install:
	go install github.com/99designs/gqlgen
	brew install mockery
	brew install golangci-lint

run:
	go run cmd/server/main.go

build:
	go build -o build/enchanted-twin cmd/server/main.go

lint:
	golangci-lint run

test:
	go test ./...

gqlgen:
	go run github.com/99designs/gqlgen generate