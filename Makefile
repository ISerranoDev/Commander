# Common tasks. Requires Go, Node and the Wails CLI
# (go install github.com/wailsapp/wails/v2/cmd/wails@latest).

LINUX_IMAGE := golang:1.23-bookworm

.PHONY: dev test build build-macos build-windows build-linux build-linux-docker

dev:
	wails dev

test:
	go test ./...

# Build for the current OS.
build:
	wails build

# Universal binary (Intel + Apple Silicon). Run on macOS.
build-macos:
	wails build -platform darwin/universal

# Cross-compiles from any OS (no CGO on Windows). Add -nsis for an installer
# (needs makensis installed).
build-windows:
	wails build -platform windows/amd64

# Run on Linux with GTK 3 + WebKit2GTK 4.1 dev packages installed
# (Ubuntu 22.04+, Debian 12+, Fedora 38+).
build-linux:
	wails build -tags webkit2_41

# Linux build from macOS/Windows inside Docker. Output: build/bin/linux/.
build-linux-docker:
	cd frontend && npm install && npm run build
	docker run --rm -v "$(CURDIR)":/src -w /src $(LINUX_IMAGE) bash -c '\
		apt-get update -qq && \
		apt-get install -y -qq libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config >/dev/null && \
		go build -buildvcs=false -tags desktop,production,webkit2_41 -o build/bin/linux/commander .'
