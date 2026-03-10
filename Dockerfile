FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /hookrelay ./cmd/hookrelay

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /hookrelay /usr/local/bin/hookrelay
COPY config/config.yaml /etc/hookrelay/config.yaml

EXPOSE 8080
ENTRYPOINT ["hookrelay"]
CMD ["--config", "/etc/hookrelay/config.yaml"]
