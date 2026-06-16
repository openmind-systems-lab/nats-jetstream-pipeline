# --- STAGE 1: Build ---
FROM golang:1.23-alpine AS builder

# Install git (required for downloading go modules)
RUN apk add --no-cache git

WORKDIR /app

# 1. Copy everything first so Go can see your imports in main.go
COPY . .

# 2. Force the download of dependencies based on the code imports
# This will update go.mod and go.sum inside the container
RUN go get github.com/lib/pq
RUN go mod tidy

# 3. Now build the static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/main .

# --- STAGE 2: Run ---
FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/
COPY --from=builder /app/main .
RUN chmod +x ./main

CMD ["./main"]