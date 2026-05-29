.PHONY: build build-router test lint tidy run

BIN_DIR := bin
BIN     := $(BIN_DIR)/bot

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN) ./cmd/bot

# Cross-compile под AX3200 (MediaTek MT7622B, ARM64).
# -s -w убирают symbol table и DWARF — бинарь ужимается ~в 2 раза.
build-router:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build -ldflags="-s -w" -o $(BIN_DIR)/bot-arm64 ./cmd/bot
	@ls -lh $(BIN_DIR)/bot-arm64

test:
	go test -race -count=1 ./...

lint:
	go vet ./...
	@command -v golangci-lint >/dev/null && golangci-lint run ./... || echo "golangci-lint not installed, skipped"

tidy:
	go mod tidy

run: build
	$(BIN)
