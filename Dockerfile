# --- Build stage ---
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install required dependencies for build
RUN apk add --no-cache git gcc g++ musl-dev make pkgconfig

# Install swagger (if needed)
RUN go install github.com/swaggo/swag/cmd/swag@latest

# Copy go.mod and go.sum
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate Swagger docs
RUN swag init -g main.go

# Build application
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-w -s" -o main .

# --- Final stage ---
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates libstdc++ libgcc

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/main .

# Create uploads directory
RUN mkdir -p uploads

EXPOSE 1212

CMD ["./main"]
