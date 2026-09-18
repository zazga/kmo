APP := kmo
BUILD_DIR := build
INSTALL_DIR ?= $(HOME)/bin
GO ?= go
GOOS ?= darwin
GOARCH ?= arm64

.PHONY: build test fmt vet install uninstall uninstall-full clean

build:
	mkdir -p $(BUILD_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -trimpath -ldflags="-s -w" -o $(BUILD_DIR)/$(APP) ./cmd/kmo

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BUILD_DIR)/$(APP) $(INSTALL_DIR)/$(APP)
	@echo "Installed $(INSTALL_DIR)/$(APP)"
	@case ":$$PATH:" in *":$(INSTALL_DIR):"*) ;; *) echo "Tip: add $(INSTALL_DIR) to your PATH." ;; esac

uninstall:
	rm -f $(INSTALL_DIR)/$(APP)
	@echo "Removed $(INSTALL_DIR)/$(APP)"

uninstall-full: uninstall
	rm -rf $(HOME)/.config/kmo $(HOME)/.local/state/kmo
	@echo "Removed KMO config and logs. Keychain PAT was intentionally kept."

clean:
	rm -rf $(BUILD_DIR)
