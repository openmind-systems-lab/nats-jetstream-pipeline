# --- STAGE 1: Build ---
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
# We build a static binary (CGO_ENABLED=0) so it runs anywhere
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/main .

# --- STAGE 2: Run ---
FROM alpine:latest

# Install CA certificates (needed for HTTPS/External APIs)
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy only the compiled binary from the builder stage
COPY --from=builder /app/main .

# Run the binary
CMD ["./main"]