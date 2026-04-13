FROM golang:1.25.1-alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/migrate ./cmd/migrate

FROM alpine:3.21 AS runtime

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata curl

COPY --from=builder /out/api /usr/local/bin/api
COPY --from=builder /out/migrate /usr/local/bin/migrate
COPY db/migrations /app/db/migrations
COPY scripts/api-entrypoint.sh /usr/local/bin/api-entrypoint.sh

RUN chmod +x /usr/local/bin/api-entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/api-entrypoint.sh"]
