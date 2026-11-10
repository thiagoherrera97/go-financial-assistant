FROM golang:1.26.1-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o finassist ./cmd/main.go

FROM alpine:3.20

# postgresql16-client: o pg_dump do backup antes das migrations precisa ser da versão do servidor (16).
RUN apk add --no-cache ca-certificates postgresql16-client

WORKDIR /app

COPY --from=builder /app/finassist .

EXPOSE 3000

CMD ["./finassist"]