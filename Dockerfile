# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cringesweeper .

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S cringesweeper && \
    adduser -u 1001 -S cringesweeper -G cringesweeper

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/cringesweeper .

# Change ownership to non-root user
RUN chown cringesweeper:cringesweeper /app/cringesweeper

# Switch to non-root user
USER cringesweeper

# Expose port 8080
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Set default command
CMD ["./cringesweeper", "server"]