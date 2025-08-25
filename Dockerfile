# Build stage
FROM golang:1.25-alpine AS builder

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments for cross-compilation
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME

# Build the binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s -extldflags '-static' -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -a -installsuffix cgo \
    -o zwfm-aeronapi .

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 aeron && \
    adduser -u 1000 -G aeron -s /bin/sh -D aeron

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/zwfm-aeronapi /app/zwfm-aeronapi

# Copy config file (if exists)
COPY --chown=aeron:aeron config.yaml* /app/

# Change ownership
RUN chown -R aeron:aeron /app

# Switch to non-root user
USER aeron

# Expose API port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

# Start API server by default
ENTRYPOINT ["/app/zwfm-aeronapi"]
CMD ["-port=8080"]