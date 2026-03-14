# Build stage
FROM golang:1.22-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go binary
# CGO_ENABLED=0 ensures a static binary which is safer for Alpine
RUN CGO_ENABLED=0 GOOS=linux go build -o battleship-backend main.go

# Run stage
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/battleship-backend .

# Expose the port (Koyeb usually overrides this, but good practice)
EXPOSE 8080

# Run the binary
CMD ["./battleship-backend"]
