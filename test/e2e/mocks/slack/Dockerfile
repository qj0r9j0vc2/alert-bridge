# Build stage
FROM golang:1.24.2-alpine AS builder

WORKDIR /build

# Copy mock service source
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-s -w' -o mock-slack .

# Runtime stage
FROM alpine:3.19

# Add ca-certificates for HTTPS requests (if needed)
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/mock-slack .

# Expose default port
EXPOSE 9091

# Set default port
ENV PORT=9091

# Run the mock service
ENTRYPOINT ["./mock-slack"]
