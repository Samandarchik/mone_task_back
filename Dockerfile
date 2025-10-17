# Build bosqichi
FROM golang:1.24.2 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO kerak bo‘lgani uchun system kutubxonalarni o‘rnatamiz
RUN apt-get update && apt-get install -y \
    build-essential \
    pkg-config \
    libde265-dev \
    libheif-dev

# CGO yoqilgan holda build
RUN CGO_ENABLED=1 GOOS=linux go build -o taskmanager .

# Minimal image
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y \
    libde265-dev libheif-dev tzdata ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /root/
COPY --from=builder /app/taskmanager .
COPY --from=builder /app/uploads ./uploads

EXPOSE 1212

ENV DSN="host=postgres user=postgres password=password dbname=taskdb port=5432 sslmode=disable"

CMD ["./taskmanager"]
