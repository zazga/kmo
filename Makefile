APP := kmo
BUILD_DIR := build
INSTALL_DIR ?= $(HOME)/bin
GO ?= go
GOOS ?= darwin
GOARCH ?= arm64
LAUNCH_LABEL := com.local.kmo-daemon
LAUNCH_PLIST := $(HOME)/Library/LaunchAgents/$(LAUNCH_LABEL).plist

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
	mkdir -p $(INSTALL_DIR) $(HOME)/Library/LaunchAgents
	cp $(BUILD_DIR)/$(APP) $(INSTALL_DIR)/$(APP)
	@printf '%s\n' '<?xml version="1.0" encoding="UTF-8"?>' '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">' '<plist version="1.0"><dict>' '<key>Label</key><string>$(LAUNCH_LABEL)</string>' '<key>ProgramArguments</key><array><string>$(INSTALL_DIR)/$(APP)</string><string>daemon</string></array>' '<key>RunAtLoad</key><true/>' '<key>KeepAlive</key><true/>' '<key>ProcessType</key><string>Background</string>' '</dict></plist>' > $(LAUNCH_PLIST)
	@launchctl bootout gui/$$(id -u) $(LAUNCH_PLIST) >/dev/null 2>&1 || true
	launchctl bootstrap gui/$$(id -u) $(LAUNCH_PLIST)
	@launchctl kickstart -k gui/$$(id -u)/$(LAUNCH_LABEL) >/dev/null 2>&1 || true
	@echo "Installed $(INSTALL_DIR)/$(APP) and started KMO daemon"
	@case ":$$PATH:" in *":$(INSTALL_DIR):"*) ;; *) echo "Tip: add $(INSTALL_DIR) to your PATH." ;; esac

uninstall:
	@launchctl bootout gui/$$(id -u) $(LAUNCH_PLIST) >/dev/null 2>&1 || true
	rm -f $(LAUNCH_PLIST) $(INSTALL_DIR)/$(APP)
	@echo "Removed KMO daemon and $(INSTALL_DIR)/$(APP)"

uninstall-full: uninstall
	rm -rf $(HOME)/.config/kmo $(HOME)/.local/state/kmo
	@echo "Removed KMO config and logs. Keychain secrets were intentionally kept."

clean:
	rm -rf $(BUILD_DIR)
