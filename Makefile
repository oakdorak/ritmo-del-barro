# Makefile for Ritmo del Barro
# Builds native binary and WebAssembly

# Default Go version
GO ?= go

# Output names
BINARY = ritmo-del-barro
WASM = build/ritmo.wasm
HTML = index.html

# Colors for output
GREEN = \033[0;32m
YELLOW = \033[1;33m
NC = \033[0m

.PHONY: all clean native wasm serve help assets

all: native wasm assets

## native: Build native binary (requires Go + CGO for Ebitengine)
native:
	@echo "$(YELLOW)Building native binary...$(NC)"
	$(GO) build -trimpath -ldflags="-s -w" -o $(BINARY)
	@echo "$(GREEN)✓ Built $(BINARY)$(NC)"

## wasm: Build WebAssembly for browser (iPad compatible)
wasm:
	@echo "$(YELLOW)Building WebAssembly...$(NC)"
	mkdir -p build
	GOOS=js GOARCH=wasm $(GO) build -trimpath -ldflags="-s -w" -o $(WASM)
	@echo "$(GREEN)✓ Built $(WASM)$(NC)"
	cp $(HTML) build/
	@# Copy wasm_exec.js from the same Go toolchain that built the wasm
	cp "$$($(GO) env GOROOT)/lib/wasm/wasm_exec.js" build/
	@echo "$(GREEN)✓ wasm_exec.js copied$(NC)"

## assets: Copy game assets to build directory
assets:
	@mkdir -p build
	@cp tileset.png build/ 2>/dev/null || true
	@cp sprites.png build/ 2>/dev/null || true
	@echo "$(GREEN)✓ Assets copied to build/$(NC)"

## serve: Serve the game locally for testing
serve:
	@echo "$(YELLOW)Serving at http://localhost:8080...$(NC)"
	cd build && python3 -m http.server 8080

## clean: Remove build artifacts
clean:
	rm -f $(BINARY)
	rm -rf build/

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
