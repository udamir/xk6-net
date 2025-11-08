FROM golang:1.24-alpine as builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make g++ musl-dev

# Copy source code and set permissions
COPY --chmod=755 build.sh .
COPY . .

# Run build script
RUN ./build.sh

# Use Alpine as the final image
FROM grafana/k6:latest

# Copy the k6 binary from the builder stage
COPY --from=builder /app/bin/k6 /usr/bin/k6