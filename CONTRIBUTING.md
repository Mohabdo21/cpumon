# Contributing

Thanks for contributing to cpumon. Small, focused PRs land fastest.

## Setup

```sh
make build   # debug build
make run     # build + run
```

Requires Go 1.26.4+ (see `go.mod`) and Linux. cpumon reads `/sys` and `/proc`, so real testing needs a physical machine, not a container.

## Development

- **Zero external deps** - stdlib only. New dependencies need a strong justification in the PR.
- **Linux only** - this project targets `/sys`, `/proc`, hwmon, and sensors.
- Keep changes small and scoped. No unrelated refactors.
- No tests exist yet - add them alongside your change if you can.

## Lint & Format

```sh
make lint   # go vet + golangci-lint
make check  # fmt + fix + lint, auto-fixes
```

CI enforces `golangci-lint` (gofumpt, goimports, gci, golines, errcheck, gosec, staticcheck, govet, unused). No exceptions - run it before pushing.

## Commits

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
fix(db): resolve connection pool leak
feat(ui): add dark mode
chore(release): bump version to 0.2.6
```

One logical change per commit. Lowercase, no trailing period.

## Pull Requests

- Keep the PR description template filled in: What & Why, Verification, Breaking Changes.
- Link the issue you close (`Closes #123`).
- Run `make lint` and verify on real hardware before requesting review.

## Release

Maintainers tag releases via the `release` workflow. Contributors don't need to handle AUR updates or releases.
