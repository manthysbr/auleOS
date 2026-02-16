
# auleOS Makefile

.PHONY: all build test gen clean

all: gen build

# Tools
OAPI_CODEGEN := $(shell go env GOPATH)/bin/oapi-codegen

# Generation
gen: gen-watchdog

gen-watchdog:
	@echo "Generating Watchdog API..."
	@mkdir -p pkg/watchdog/api
	$(OAPI_CODEGEN) -config specs/oapi-codegen-watchdog.yaml specs/watchdog-api.yaml

# Build
build:
	go build -o bin/aule-kernel ./cmd/aule-kernel
	go build -o bin/aule-watchdog ./pkg/watchdog

# Test
test:
	go test ./...

clean:
	rm -rf bin/
