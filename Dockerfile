# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o server cmd/server/main.go

# Run stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/server .

# Copy configuration and public files
COPY --from=builder /app/configs ./configs
COPY --from=builder /app/public ./public

# Create directories for logs and tmp
RUN mkdir -p logs tmp

# Set environment variables
ENV ENV=prod

# Expose port
EXPOSE 5100

# Run the application
CMD ["./server"]
