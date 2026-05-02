FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bot ./cmd/bot

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bot /usr/local/bin/bot
COPY config.yaml /etc/bot/config.yaml

ENTRYPOINT ["bot"]
CMD ["-config", "/etc/bot/config.yaml"]
