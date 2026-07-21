APP_NAME := moistchat
GO := go
GOFLAGS := -ldflags "-X moistchat/internal/network.AuthKey=$(TAILSCALE_AUTH_KEY)"

.PHONY: all build run clean cross-compile

all: build

build:
	$(GO) build $(GOFLAGS) -o bin/$(APP_NAME) ./cmd/$(APP_NAME)

run:
	$(GO) run $(GOFLAGS) ./cmd/$(APP_NAME)

dev:
	$(GO) run ./cmd/$(APP_NAME)

lobby:
	$(GO) build $(GOFLAGS) -o bin/lobby ./cmd/lobby

lobby-run:
	$(GO) run $(GOFLAGS) ./cmd/lobby

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

