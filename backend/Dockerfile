# ============================================================
# Stage 1: Build Go binary
# ============================================================
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache ca-certificates git tzdata

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOCACHE=/tmp/.cache/go-build

RUN go build -trimpath \
    -ldflags="-s -w -extldflags '-static' \
    -X main.buildVersion=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev') \
    -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /bin/bianzhong-server ./main.go

# ============================================================
# Stage 2: Runtime image (distroless-style static)
# ============================================================
FROM alpine:3.19 AS runtime

RUN apk add --no-cache ca-certificates tzdata curl \
    && rm -rf /var/cache/apk/*

ENV TZ=Asia/Shanghai \
    GIN_MODE=release \
    PORT=8080

WORKDIR /app

COPY --from=builder /bin/bianzhong-server /app/bianzhong-server
COPY config/ /app/config/
COPY ../frontend /app/frontend/
COPY ../sql/init.sql /app/sql/init.sql

RUN addgroup -S appgroup \
    && adduser -S appuser -G appgroup \
    && chown -R appuser:appgroup /app \
    && chmod +x /app/bianzhong-server

USER appuser

HEALTHCHECK --interval=30s --timeout=5s --retries=3 --start-period=15s \
    CMD curl -f http://localhost:8080/api/dashboard/stats || exit 1

EXPOSE 8080

ENTRYPOINT ["/app/bianzhong-server"]
