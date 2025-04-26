FROM golang:1.24 AS builder
WORKDIR /app
COPY . .
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/server .

RUN chmod 777 /app/server

RUN adduser -D -u 1000 appuser
USER appuser


CMD ["/app/server"]

