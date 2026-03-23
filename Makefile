EXECUTABLE=dev_gateway_dns
WINDOWS_AMD64=$(EXECUTABLE)_windows_amd64.exe
WINDOWS_ARM64=$(EXECUTABLE)_windows_arm64.exe
LINUX_AMD64=$(EXECUTABLE)_linux_amd64
LINUX_ARM64=$(EXECUTABLE)_linux_arm64
DARWIN_AMD64=$(EXECUTABLE)_macos_amd64
DARWIN_ARM64=$(EXECUTABLE)_macos_arm64
VERSION=v1.0.0

LDFLAGS=-s -w -X main.version=$(VERSION)
BIN_DIR=bin

# OS detection for shell commands
ifeq ($(OS),Windows_NT)
    MKDIR = powershell -Command "New-Item -ItemType Directory -Force -Path $(BIN_DIR) | Out-Null"
    RM = powershell -Command "Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $(BIN_DIR)"
else
    MKDIR = mkdir -p $(BIN_DIR)
    RM = rm -rf $(BIN_DIR)
endif

.PHONY: all build windows linux darwin clean prepare

all: build

build: windows linux darwin

prepare:
	$(MKDIR)

windows: prepare $(WINDOWS_AMD64) $(WINDOWS_ARM64)

linux: prepare $(LINUX_AMD64) $(LINUX_ARM64)

darwin: prepare $(DARWIN_AMD64) $(DARWIN_ARM64)

# Build target macro
define build-target
$(1): export GOOS=$(2)
$(1): export GOARCH=$(3)
$(1):
	go build -o $(BIN_DIR)/$(1) -ldflags="$(LDFLAGS)" main.go
endef

$(eval $(call build-target,$(WINDOWS_AMD64),windows,amd64))
$(eval $(call build-target,$(WINDOWS_ARM64),windows,arm64))
$(eval $(call build-target,$(LINUX_AMD64),linux,amd64))
$(eval $(call build-target,$(LINUX_ARM64),linux,arm64))
$(eval $(call build-target,$(DARWIN_AMD64),darwin,amd64))
$(eval $(call build-target,$(DARWIN_ARM64),darwin,arm64))

clean:
	$(RM)
