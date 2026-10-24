BINARY = cpumon
BUILD_DIR = build
AUR_REPO = ssh://aur@aur.archlinux.org/cpumon.git
AUR_DIR = /tmp/cpumon-aur
VERSION = $(shell grep 'const releaseVersion' main.go | cut -d'"' -f2)
VER_CLEAN = $(patsubst v%,%,$(or $(RELEASE_VERSION),$(VERSION)))
COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GOAMD64 ?= v3

.PHONY: build build-optimized run install clean lint check test release aur-clone aur-update aur-publish

build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="-X main.version=$(VERSION) -X main.commit=$(COMMIT)" -o $(BUILD_DIR)/$(BINARY) .

build-optimized:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOAMD64=$(GOAMD64) go build \
		-trimpath \
		-ldflags="-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)" \
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

test:
	go test ./...
	python3 scripts/release_test.py

check:
	go fmt ./...
	go fix ./...
	go vet ./...
	$(MAKE) test
	golangci-lint fmt
	golangci-lint run --fix

# --- Release ---

release:
	$(if $(filter command line,$(origin VERSION)),$(error Use RELEASE_VERSION=vX.Y.Z, not VERSION))
	@scripts/release.sh "$(RELEASE_VERSION)"

# --- AUR ---

aur-clone:
	@if [ ! -d "$(AUR_DIR)" ]; then \
		git clone $(AUR_REPO) $(AUR_DIR); \
	else \
		echo "AUR repo already cloned at $(AUR_DIR)"; \
	fi

aur-update: aur-clone
	@cd $(AUR_DIR) && git pull
	@set -e; \
	CURRENT_VER=$$(grep '^pkgver=' $(AUR_DIR)/PKGBUILD | cut -d= -f2); \
	CURRENT_REL=$$(grep '^pkgrel=' $(AUR_DIR)/PKGBUILD | cut -d= -f2); \
	NEW_VER=$(VER_CLEAN); \
	cp -f aur/PKGBUILD aur/cpumon.install $(AUR_DIR)/; \
	sed -i "s/^pkgver=.*/pkgver=$$NEW_VER/" $(AUR_DIR)/PKGBUILD; \
	if [ "$$CURRENT_VER" != "$$NEW_VER" ]; then \
		echo "Version changed: $$CURRENT_VER -> $$NEW_VER"; \
		sed -i "s/^pkgrel=.*/pkgrel=1/" $(AUR_DIR)/PKGBUILD; \
		echo "pkgrel reset to 1"; \
	else \
		echo "Version unchanged: $$CURRENT_VER"; \
		printf 'Increment pkgrel? (y/n): '; \
		read -r inc; \
		if [ "$$inc" = "y" ]; then \
			NEW_REL=$$((CURRENT_REL + 1)); \
			sed -i "s/^pkgrel=.*/pkgrel=$$NEW_REL/" $(AUR_DIR)/PKGBUILD; \
			echo "pkgrel incremented to $$NEW_REL"; \
		else \
			sed -i "s/^pkgrel=.*/pkgrel=$$CURRENT_REL/" $(AUR_DIR)/PKGBUILD; \
			echo "pkgrel left as $$CURRENT_REL"; \
		fi \
	fi
	@echo "Updating sha256sums..."
	@cd $(AUR_DIR) && \
		SUMS=$$(makepkg -g) && \
		printf '%s\n' "$$SUMS" | grep -Eq "^sha256sums=\('[0-9a-f]{64}'\)$$" && \
		sed -i "s/^sha256sums=.*/$$SUMS/" PKGBUILD
	@set -e; cd $(AUR_DIR); trap 'rm -f .SRCINFO.tmp' EXIT; \
		makepkg --printsrcinfo > .SRCINFO.tmp; mv .SRCINFO.tmp .SRCINFO
	@cp $(AUR_DIR)/PKGBUILD $(AUR_DIR)/.SRCINFO aur/
	@echo "PKGBUILD and .SRCINFO updated for $(VER_CLEAN)"

aur-publish: aur-update
	@cd $(AUR_DIR) && \
		if ! git diff --quiet PKGBUILD .SRCINFO cpumon.install; then \
			git add PKGBUILD .SRCINFO cpumon.install && \
			git commit -m "Update to $(VER_CLEAN)" && \
			git push && \
			echo "Published cpumon to AUR"; \
		else \
			echo "No changes to commit."; \
		fi
