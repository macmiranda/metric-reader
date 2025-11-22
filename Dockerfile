FROM golang:1.24-bookworm AS builder

WORKDIR /app
COPY . .

# Install build dependencies
RUN apt-get update && apt-get install -y \
    gcc-aarch64-linux-gnu \
    g++-aarch64-linux-gnu \
    && rm -rf /var/lib/apt/lists/*

# Download dependencies
RUN go mod download

# Build the main application
RUN go build -o metric-reader

# Build plugins
RUN mkdir -p /app/plugins
RUN go build -buildmode=plugin -o /app/plugins/log_action.so plugins/log_action/log_action.go
RUN go build -buildmode=plugin -o /app/plugins/file_action.so plugins/file_action/file_action.go
RUN go build -buildmode=plugin -o /app/plugins/efs_emergency.so plugins/efs_emergency/efs_emergency.go

FROM alpine:latest

# Install runtime dependencies for plugins
RUN apk add --no-cache libc6-compat

WORKDIR /app

# Copy the main application
COPY --from=builder /app/metric-reader .

# Create plugins directory and copy plugins
RUN mkdir -p /app/plugins
COPY --from=builder /app/plugins/*.so /app/plugins/

# Set default plugin directory
ENV PLUGIN_DIR=/app/plugins

CMD ["./metric-reader"]
