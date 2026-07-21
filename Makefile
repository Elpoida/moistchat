APP_NAME := moistchat
GO := go
GOFLAGS := -ldflags "-X moistchat/internal/network.AuthKey=$(TAILSCALE_AUTH_KEY)"

# The app only uses libopus's live encode/decode API, never the opusfile
# stream API, so build without opusfile everywhere. This drops the libopusfile
# link dependency on every platform (only libopus is needed).
TAGS := nolibopusfile

.PHONY: all build build-macos run dev lobby lobby-run clean clean-state cross-compile

all: build

build:
	$(GO) build -tags "$(TAGS)" $(GOFLAGS) -o bin/$(APP_NAME) ./cmd/$(APP_NAME)

# Self-contained macOS build: libopus is statically linked, so the resulting
# binary has NO Homebrew/dylib dependency and runs on any Mac of the same
# architecture without `brew install opus`.
#
# Requires (build machine only): brew install opus pkg-config
# We override pkg-config with a generated .pc that points at libopus.a instead
# of -lopus, and force a clean rebuild (-a) so a previously cached dynamic build
# of the opus package can't leak the dylib into the final link.
build-macos:
	@command -v pkg-config >/dev/null 2>&1 || { echo "error: pkg-config not found — run: brew install pkg-config"; exit 1; }
	@pkg-config --exists opus || { echo "error: libopus not found — run: brew install opus"; exit 1; }
	@libdir="$$(pkg-config --variable=libdir opus)"; \
	 if [ ! -f "$$libdir/libopus.a" ]; then \
	   echo "error: static libopus.a not found in $$libdir"; exit 1; \
	 fi; \
	 pcdir="$$(mktemp -d)"; \
	 printf 'Name: opus\nDescription: opus (static)\nVersion: %s\nCflags: %s\nLibs: %s/libopus.a\n' \
	   "$$(pkg-config --modversion opus)" "$$(pkg-config --cflags opus)" "$$libdir" > "$$pcdir/opus.pc"; \
	 echo "Building self-contained macOS binary (static libopus)..."; \
	 CGO_ENABLED=1 PKG_CONFIG_PATH="$$pcdir" \
	   $(GO) build -a -tags "$(TAGS)" $(GOFLAGS) -o bin/$(APP_NAME) ./cmd/$(APP_NAME); \
	 status=$$?; rm -rf "$$pcdir"; \
	 if [ $$status -eq 0 ]; then \
	   echo "built bin/$(APP_NAME)"; \
	   echo "verify self-contained:  otool -L bin/$(APP_NAME) | grep -i opus   (should print nothing)"; \
	 fi; \
	 exit $$status

run:
	$(GO) run -tags "$(TAGS)" $(GOFLAGS) ./cmd/$(APP_NAME)

dev:
	$(GO) run -tags "$(TAGS)" ./cmd/$(APP_NAME)

lobby:
	$(GO) build -tags "$(TAGS)" $(GOFLAGS) -o bin/lobby ./cmd/lobby

lobby-run:
	$(GO) run -tags "$(TAGS)" $(GOFLAGS) ./cmd/lobby

clean:
	rm -rf bin/

clean-state:
	@rm -rf "$(HOME)/.config/tsnet-moistchat"
	@echo "Cleared tsnet state directory ($(HOME)/.config/tsnet-moistchat)"

cross-compile:
	GOOS=linux GOARCH=amd64   $(GO) build $(GOFLAGS) -o bin/$(APP_NAME)-linux-amd64   ./cmd/$(APP_NAME)
	GOOS=darwin GOARCH=amd64  $(GO) build $(GOFLAGS) -o bin/$(APP_NAME)-darwin-amd64  ./cmd/$(APP_NAME)
	GOOS=darwin GOARCH=arm64  $(GO) build $(GOFLAGS) -o bin/$(APP_NAME)-darwin-arm64  ./cmd/$(APP_NAME)
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o bin/$(APP_NAME)-windows-amd64.exe ./cmd/$(APP_NAME)
