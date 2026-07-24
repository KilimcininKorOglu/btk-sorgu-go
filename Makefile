.PHONY: build clean test test-race test-cover test-verbose bench run fmt vet lint help

BINARY_NAME=btk-sorgu
BUILD_DIR=bin
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-s -w -X 'main.version=$(VERSION)' -X 'main.commit=$(COMMIT)' -X 'main.buildDate=$(BUILD_DATE)'

build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)_$(GOOS)_$(GOARCH) .

clean:
	rm -rf $(BUILD_DIR)
	go clean

test:
	go test ./...

test-race:
	go test -race ./...

test-cover:
	go test -cover ./...

test-verbose:
	go test -v ./...

bench:
	go test -bench=. -benchmem ./...

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)_$(GOOS)_$(GOARCH)

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet

help:
	@echo "Kullanılabilir hedefler:"
	@echo "  build        - Binary'yi bin/ dizinine derler"
	@echo "  clean        - Build çıktısını kaldırır"
	@echo "  test         - Tüm testleri çalıştırır"
	@echo "  test-race    - Testleri race detector ile çalıştırır"
	@echo "  test-cover   - Testleri coverage ile çalıştırır"
	@echo "  test-verbose - Testleri ayrıntılı çıktıyla çalıştırır"
	@echo "  bench        - Benchmark'ları çalıştırır"
	@echo "  run          - Derler ve sunucuyu başlatır"
	@echo "  fmt          - Kodu formatlar"
	@echo "  vet          - go vet çalıştırır"
	@echo "  lint         - fmt ve vet çalıştırır"
