# Stage 1: Build watchdog
FROM golang:1.25-alpine AS watchdog-builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY pkg/watchdog ./pkg/watchdog
COPY specs/watchdog-api.yaml specs/oapi-codegen-watchdog.yaml ./specs/

# Build watchdog binary
RUN go build -o watchdog ./pkg/watchdog/cmd

# Stage 2: Runtime with latest Ollama (v0.16+, supports image generation)
FROM ollama/ollama:latest

# Install curl for health checks
RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*

# Copy watchdog binary
COPY --from=watchdog-builder /build/watchdog /usr/local/bin/watchdog
RUN chmod +x /usr/local/bin/watchdog

# Set Ollama models directory to shared location
ENV OLLAMA_MODELS=/opt/ollama/models
RUN mkdir -p /opt/ollama/models

# Create non-root user
RUN useradd -m -u 1000 aule

# Create workspace and set permissions
RUN mkdir -p /mnt/aule/workspace && \
    chown -R aule:aule /mnt/aule && \
    chown -R aule:aule /opt/ollama

# Expose watchdog port
EXPOSE 8080

# Entrypoint script to start Ollama daemon + watchdog
COPY workers/image-gen/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER aule
ENTRYPOINT ["/entrypoint.sh"]
