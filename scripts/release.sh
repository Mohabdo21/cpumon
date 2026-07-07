#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-}"
FORCE=false

# Normalize: ensure v prefix
if [ -n "$VERSION" ] && [ "${VERSION:0:1}" != "v" ]; then
	VERSION="v$VERSION"
fi

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { echo -e "${GREEN}[INFO]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() {
	echo -e "${RED}[ERROR]${NC} $*" >&2
	exit 1
}

preflight_checks() {
	if ! git diff --quiet HEAD; then
		error "Working tree has uncommitted changes. Commit or stash first."
	fi
	local branch
	branch=$(git rev-parse --abbrev-ref HEAD)
	if [ "$branch" != "main" ]; then
		error "Releases must be cut from 'main', currently on '$branch'"
	fi
	if ! command -v gh &>/dev/null; then
		error "'gh' CLI not found. Install it: https://cli.github.com/"
	fi
	git fetch --tags origin
	info "Pre-flight checks passed"
}

resolve_version() {
	local latest_tag
	latest_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")

	if [ -z "$VERSION" ]; then
		local base="${latest_tag#v}"
		local major minor patch
		IFS='.' read -r major minor patch <<<"$base"
		: "${patch:=0}"
		VERSION="v$major.$minor.$((patch + 1))"
		info "No version specified. Suggesting: $VERSION (latest was $latest_tag)"
		read -r -p "Press Enter to use $VERSION, or type a different version: " input
		if [ -n "$input" ]; then
			VERSION="$input"
		fi
	fi

	if ! echo "$VERSION" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
		error "Version must match 'vX.Y.Z' format, got: $VERSION"
	fi

	if git rev-parse -q --verify "refs/tags/$VERSION" &>/dev/null; then
		warn "Tag $VERSION already exists locally."
		if git ls-remote --tags origin "$VERSION" 2>/dev/null | grep -q .; then
			warn "Tag $VERSION also exists on remote."
		fi
		read -r -p "Re-release (force replace)? [y/N] " confirm
		if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
			error "Aborted. Use a different version."
		fi
		FORCE=true
	fi

	if [ "$(echo -e "$latest_tag\n$VERSION" | sort -V | tail -1)" != "$VERSION" ]; then
		warn "$VERSION is lower than the latest tag ($latest_tag)"
		read -r -p "Continue anyway? [y/N] " confirm
		if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
			error "Aborted."
		fi
	fi

	info "Releasing version: $VERSION"
}

generate_changelog() {
	local prev_tag
	prev_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "")

	if [ -z "$prev_tag" ]; then
		CHANGELOG=$(git log --oneline --format="- %s (%h)" --reverse 2>/dev/null || echo "No commits found.")
		info "First release. Including all commits."
	else
		CHANGELOG=$(git log --oneline --format="- %s (%h)" "$prev_tag"..HEAD 2>/dev/null || echo "No new commits since $prev_tag.")
	fi

	if [ -z "$CHANGELOG" ] || [ "$CHANGELOG" = "No new commits since $prev_tag." ]; then
		warn "$CHANGELOG"
		read -r -p "No new commits. Continue anyway? [y/N] " confirm
		if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
			error "Aborted."
		fi
	fi

	CHANGELOG_FILE=$(mktemp)
	{
		echo "# Release $VERSION"
		echo ""
		echo "$CHANGELOG"
		echo ""
	} >"$CHANGELOG_FILE"
}

bump_version() {
	local ver_no_v="${VERSION#v}"
	local current_ver
	current_ver=$(grep 'const version' main.go | cut -d'"' -f2)

	info "Bumping version: $current_ver -> $ver_no_v"

	sed -i "s/const version = \"$current_ver\"/const version = \"$ver_no_v\"/" main.go
	sed -i "s/^pkgver=.*/pkgver=$ver_no_v/" aur/PKGBUILD
	sed -i "s/^pkgrel=.*/pkgrel=1/" aur/PKGBUILD
	sed -i "s/^pkgver =.*/pkgver = $ver_no_v/" aur/.SRCINFO
	sed -i "s/^pkgrel =.*/pkgrel = 1/" aur/.SRCINFO

	git add main.go aur/PKGBUILD aur/.SRCINFO
	git commit -m "chore: bump version to $ver_no_v"
	info "Version bump committed"
}

build_binaries() {
	info "Building binaries..."
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-trimpath -ldflags="-s -w" -buildmode=pie \
		-o dist/cpumon-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
		-trimpath -ldflags="-s -w" -buildmode=pie \
		-o dist/cpumon-linux-arm64 .
	cd dist && sha256sum ./* >checksums.txt && cd ..
	info "Binaries built in dist/:"
	ls -lh dist/
}

tag_and_push() {
	local tag_opts="-a"
	local push_opts=""
	if [ "$FORCE" = true ]; then
		tag_opts="-fa"
		push_opts="--force"
	fi

	git push origin HEAD
	git tag $tag_opts "$VERSION" -m "$VERSION"
	git push origin "$VERSION" $push_opts
	info "Tag $VERSION pushed to origin"
}

update_aur_checksums() {
	local ver_no_v="${VERSION#v}"
	info "Computing sha256sums from GitHub tarball..."
	local sha
	sha=$(curl -sL "https://github.com/Mohabdo21/cpumon/archive/$VERSION.tar.gz" | sha256sum | cut -d' ' -f1)

	sed -i "s/^sha256sums=.*/sha256sums=('$sha')/" aur/PKGBUILD
	sed -i "s/^sha256sums =.*/sha256sums = $sha/" aur/.SRCINFO

	git add aur/PKGBUILD aur/.SRCINFO
	git commit -m "chore: update AUR checksums for $VERSION"
	git push origin main
	info "AUR checksums committed and pushed"
}

create_release() {
	if [ "$FORCE" = true ]; then
		if gh release view "$VERSION" --json id &>/dev/null 2>&1; then
			warn "Release $VERSION already exists on GitHub. Deleting..."
			gh release delete "$VERSION" --yes
		fi
	fi

	gh release create "$VERSION" \
		dist/cpumon-linux-amd64 \
		dist/cpumon-linux-arm64 \
		dist/checksums.txt \
		--title "$VERSION" \
		--notes-file "$CHANGELOG_FILE"

	rm -f "$CHANGELOG_FILE"
	info "Release $VERSION created successfully!"

	local repo
	repo=$(git remote get-url origin 2>/dev/null | sed 's|.*github.com[:/]||; s|\.git$||')
	info "URL: https://github.com/$repo/releases/tag/$VERSION"
}

publish_aur() {
	info "Publishing to AUR..."
	make aur-publish
	info "AUR publish complete"
}

main() {
	echo ""
	echo "========================================"
	echo "  cpumon release script"
	echo "========================================"
	echo ""

	preflight_checks
	resolve_version
	generate_changelog
	bump_version
	build_binaries
	tag_and_push
	update_aur_checksums
	create_release
	publish_aur

	echo ""
	echo "========================================"
	echo "  Release $VERSION complete!"
	echo "========================================"
}

main
