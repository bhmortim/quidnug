# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /quidnug ./cmd/quidnug

FROM alpine:3.20

RUN apk --no-cache add ca-certificates wget \
    && addgroup -S -g 10001 quidnug \
    && adduser  -S -u 10001 -G quidnug -H -s /sbin/nologin quidnug

WORKDIR /app

COPY --from=builder --chown=quidnug:quidnug /quidnug /app/quidnug

USER quidnug:quidnug

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q --spider http://127.0.0.1:8080/api/health || exit 1

ENTRYPOINT ["/app/quidnug"]
