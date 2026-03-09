BINARY = cpumon
BUILD_DIR = build
AUR_REPO = ssh://aur@aur.archlinux.org/cpumon.git
AUR_DIR = /tmp/cpumon-aur
VERSION = $(shell grep 'const version' main.go | cut -d'"' -f2)

.PHONY: build build-optimized run install clean lint aur-clone aur-update aur-publish

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) .

build-optimized:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -gcflags="-l=4" -o $(BUILD_DIR)/$(BINARY) .

run: build
	@$(BUILD_DIR)/$(BINARY)

install: build-optimized
	sudo cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY)

clean:
	rm -rf $(BUILD_DIR)

lint:
	go vet ./...
	golangci-lint run

# --- AUR ---

aur-clone:
	@if [ ! -d "$(AUR_DIR)" ]; then \
		git clone $(AUR_REPO) $(AUR_DIR); \
	else \
		echo "AUR repo already cloned at $(AUR_DIR)"; \
	fi

aur-update: aur-clone
	@echo "Updating AUR package to v$(VERSION)..."
	@sed -i "s/^pkgver=.*/pkgver=$(VERSION)/" $(AUR_DIR)/PKGBUILD
	@sed -i "s/^pkgrel=.*/pkgrel=1/" $(AUR_DIR)/PKGBUILD
	@cp aur/PKGBUILD $(AUR_DIR)/PKGBUILD
	@sed -i "s/^pkgver=.*/pkgver=$(VERSION)/" $(AUR_DIR)/PKGBUILD
	@cd $(AUR_DIR) && SHA=$$(makepkg -g 2>/dev/null | grep -oP "'\K[^']+" | head -1) && \
		sed -i "s/^sha256sums=.*/sha256sums=('$$SHA')/" PKGBUILD
	@cd $(AUR_DIR) && makepkg --printsrcinfo > .SRCINFO
	@echo "PKGBUILD and .SRCINFO updated for v$(VERSION)"

aur-publish: aur-update
	@cd $(AUR_DIR) && \
		git add PKGBUILD .SRCINFO && \
		git commit -m "Update to $(VERSION)" && \
		git push
	@echo "Published cpumon v$(VERSION) to AUR"
