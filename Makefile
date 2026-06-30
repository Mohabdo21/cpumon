BINARY = cpumon
BUILD_DIR = build
AUR_REPO = ssh://aur@aur.archlinux.org/cpumon.git
AUR_DIR = /tmp/cpumon-aur
VERSION = $(shell grep 'const version' main.go | cut -d'"' -f2)
GOAMD64 ?= v3

.PHONY: build build-optimized run install clean lint release aur-clone aur-update aur-publish

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) .

build-optimized:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOAMD64=$(GOAMD64) go build \
		-trimpath \
		-ldflags="-s -w" \
		-buildmode=pie \
		-o $(BUILD_DIR)/$(BINARY) .

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

# --- Release ---

release:
	@if [ -z "$(VERSION)" ]; then \
		echo "Usage: make release VERSION=x.y.z"; \
		exit 1; \
	fi; \
	CURRENT_VER=$$(grep 'const version' main.go | cut -d'"' -f2); \
	echo ""; \
	echo "cpumon v$$CURRENT_VER -> v$(VERSION)"; \
	read -p "Proceed? [y/N] " ok; \
	if [ "$$ok" != "y" ]; then exit 1; fi; \
	echo ""; \
	echo "==> Bumping version in main.go and aur/PKGBUILD"; \
	sed -i "s/const version = \"$$CURRENT_VER\"/const version = \"$(VERSION)\"/" main.go; \
	sed -i "s/^pkgver=.*/pkgver=$(VERSION)/" aur/PKGBUILD; \
	sed -i "s/^pkgrel=.*/pkgrel=1/" aur/PKGBUILD; \
	git add main.go aur/PKGBUILD; \
	git commit -m "chore: bump version to $(VERSION)"; \
	echo ""; \
	echo "==> Tagging v$(VERSION)"; \
	git tag "v$(VERSION)"; \
	echo ""; \
	echo "==> Pushing to GitHub"; \
	git push origin main "v$(VERSION)"; \
	echo ""; \
	echo "==> Setting sha256sums to SKIP (git source)"; \
	sed -i "s/^sha256sums=.*/sha256sums=('SKIP')/" aur/PKGBUILD; \
	git add aur/PKGBUILD; \
	git commit -m "chore: update AUR PKGBUILD checksums for v$(VERSION)"; \
	git push origin main; \
	echo ""; \
	echo "==> Publishing to AUR"; \
	$(MAKE) aur-publish; \
	echo ""; \
	echo "Release v$(VERSION) complete."

# --- AUR ---

aur-clone:
	@if [ ! -d "$(AUR_DIR)" ]; then \
		git clone $(AUR_REPO) $(AUR_DIR); \
	else \
		echo "AUR repo already cloned at $(AUR_DIR)"; \
	fi

aur-update: aur-clone
	@cd $(AUR_DIR) && git pull
	@CURRENT_VER=$$(grep '^pkgver=' $(AUR_DIR)/PKGBUILD | cut -d= -f2); \
	CURRENT_REL=$$(grep '^pkgrel=' $(AUR_DIR)/PKGBUILD | cut -d= -f2); \
	NEW_VER=$(VERSION); \
	if [ "$$CURRENT_VER" != "$$NEW_VER" ]; then \
		echo "Version changed: $$CURRENT_VER -> $$NEW_VER"; \
		sed -i "s/^pkgver=.*/pkgver=$$NEW_VER/" $(AUR_DIR)/PKGBUILD; \
		sed -i "s/^pkgrel=.*/pkgrel=1/" $(AUR_DIR)/PKGBUILD; \
		echo "pkgrel reset to 1"; \
	else \
		echo "Version unchanged: $$CURRENT_VER"; \
		read -p "Increment pkgrel? (y/n): " inc; \
		if [ "$$inc" = "y" ]; then \
			NEW_REL=$$((CURRENT_REL + 1)); \
			sed -i "s/^pkgrel=.*/pkgrel=$$NEW_REL/" $(AUR_DIR)/PKGBUILD; \
			echo "pkgrel incremented to $$NEW_REL"; \
		else \
			echo "pkgrel left as $$CURRENT_REL"; \
		fi \
	fi
	@echo "Updating sha256sums..."
	@cd $(AUR_DIR) && \
		if grep -q "^source=.*git+" PKGBUILD; then \
			sed -i "s/^sha256sums=.*/sha256sums=('SKIP')/" PKGBUILD; \
		else \
			SHA=$$(makepkg -g 2>/dev/null | grep -oP "'\K[^']+" | head -1) && \
			sed -i "s/^sha256sums=.*/sha256sums=('$$SHA')/" PKGBUILD; \
		fi
	@cd $(AUR_DIR) && makepkg --printsrcinfo > .SRCINFO
	@echo "PKGBUILD and .SRCINFO updated for $(VERSION)"

aur-publish: aur-update
	@cd $(AUR_DIR) && \
		if ! git diff --quiet PKGBUILD .SRCINFO; then \
			git add PKGBUILD .SRCINFO && \
			git commit -m "Update to $(VERSION)" && \
			git push && \
			echo "Published cpumon to AUR"; \
		else \
			echo "No changes to commit."; \
		fi
