# Build stage
FROM node:20-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ .
RUN npm run build

# Go build stage
FROM golang:1.21-alpine AS backend-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /filetransmitter ./cmd/server

# Final stage
FROM alpine:3.19
WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata

# Copy binary
COPY --from=backend-builder /filetransmitter /app/filetransmitter

# Copy frontend
COPY --from=frontend-builder /app/web/dist /app/web/dist

# Copy config
COPY config.yaml /app/config.yaml

# Create temp directory
RUN mkdir -p /app/tmp

# Expose ports
EXPOSE 8080 37021/udp

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q --spider http://localhost:8080/health || exit 1

# Run
ENTRYPOINT ["/app/filetransmitter"]
CMD ["--config", "/app/config.yaml"]