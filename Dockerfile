# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache make git

# First, copy only the pkg directory and go.mod/go.sum
COPY pkg/ ./pkg/
COPY go.mod go.sum ./

# Set GOTOOLCHAIN to match the version in go.mod
ENV GOTOOLCHAIN=local

# Download dependencies
RUN go mod download

# Now copy the rest of the code
COPY . .

# Build the application
RUN make build

# Final stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies and postgresql-client
RUN apk add --no-cache postgresql-client

# Copy the binary from builder
COPY --from=builder /app/parity-server .
COPY --from=builder /app/.env .

# Copy the entrypoint script
COPY docker-entrypoint.sh .
RUN chmod +x docker-entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["./docker-entrypoint.sh"]