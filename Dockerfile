# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o lightfile6-insights-gateway ./cmd/gateway

# Final stage
FROM alpine:3.19

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

# Create cache directory with proper permissions
RUN mkdir -p /var/lib/lightfile6-insights-gateway && \
    chown -R appuser:appgroup /var/lib/lightfile6-insights-gateway

# Copy binary from builder
COPY --from=builder /app/lightfile6-insights-gateway /usr/local/bin/

# Copy example config
COPY config.example.yml /etc/lightfile6/config.example.yml

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["lightfile6-insights-gateway"]

# Default arguments
CMD ["-p", "8080", "-c", "/etc/lightfile6/config.yml"]