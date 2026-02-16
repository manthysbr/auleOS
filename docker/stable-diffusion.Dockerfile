FROM ubuntu:22.04

# Install dependencies
RUN apt-get update && apt-get install -y \
    wget \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Download stable-diffusion.cpp binary (CPU-only for now, CUDA compile from source later)
WORKDIR /app
RUN wget https://github.com/leejet/stable-diffusion.cpp/releases/download/master-504-636d3cb/sd-master-636d3cb-bin-linux-x64.zip \
    && apt-get update && apt-get install -y unzip && rm -rf /var/lib/apt/lists/* \
    && unzip sd-master-636d3cb-bin-linux-x64.zip \
    && mv sd /app/sd \
    && chmod +x /app/sd \
    && rm -rf sd-master-636d3cb-bin-linux-x64.zip

# Create workspace and models dirs
RUN mkdir -p /workspace /models

CMD ["/app/sd", "--help"]
