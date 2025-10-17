# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install required dependencies
RUN apk add --no-cache git gcc g++ musl-dev

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/main .

# Create uploads directory
RUN mkdir -p uploads

# Expose port
EXPOSE 1212

# Run application
CMD ["./main"]