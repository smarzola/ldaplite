# Build stage
FROM golang:1.25-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /build

# Install build dependencies (including Node.js for Tailwind CSS)
RUN apk add --no-cache git nodejs npm

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy package files and install Node dependencies
COPY package*.json ./
RUN npm ci

# Copy source
COPY . .

# Build Tailwind CSS (generates internal/web/static/output.css)
RUN npm run build:css

# Build static binary (pure Go, no CGO needed for modernc.org/sqlite)
# The binary embeds the generated CSS via //go:embed
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s -extldflags '-static' -X main.version=${VERSION}" \
    -o ldaplite \
    ./cmd/ldaplite

# Runtime stage - distroless for minimal image
FROM gcr.io/distroless/static-debian12:nonroot

# Copy binary (migrations are embedded in the binary)
COPY --from=builder /build/ldaplite /usr/local/bin/ldaplite

# Use non-root user (distroless nonroot UID: 65532)
USER 65532:65532

# Data directory (mount volume here for persistence)
VOLUME ["/data"]

# Expose LDAP port
EXPOSE 3389

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/ldaplite", "healthcheck"]

# Entry point
ENTRYPOINT ["/usr/local/bin/ldaplite"]
CMD ["server"]
