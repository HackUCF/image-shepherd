# -----------------------------------------------------------------------------
# Build stage
FROM golang:1.23-alpine AS build
WORKDIR /app

# Install git and ca-certificates for go modules
RUN apk add --no-cache git ca-certificates

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -o image-shepherd ./cmd/image-shepherd

# -----------------------------------------------------------------------------
# Runtime stage
FROM ubuntu:24.04 AS runtime

# Create non-root user
RUN groupadd -r shepherd && useradd -r -g shepherd shepherd

# Install runtime dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    qemu-utils \
    ca-certificates\
    xz-utils  tar && \
    rm -rf /var/lib/apt/lists/* && \
    apt-get clean

# Create app directory
WORKDIR /opt/image-shepherd

# Copy binary from build stage
COPY --from=build /app/image-shepherd .

# Set ownership and permissions
RUN chown -R shepherd:shepherd /opt/image-shepherd && \
    chmod +x image-shepherd

# Switch to non-root user
USER shepherd

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/opt/image-shepherd/image-shepherd", "--version"]

# Set entrypoint and default command
ENTRYPOINT ["/opt/image-shepherd/image-shepherd"]
CMD ["-no-color", "-verbose"]
