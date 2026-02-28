# Releasing repoinjector

How to build versioned binaries and publish them as GitHub releases.

## Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [GitHub CLI (`gh`)](https://cli.github.com/)
- Authenticated with GitHub: `gh auth login`

## 1. Tag a version

The Makefile already injects the version from `git describe --tags` into the binary via ldflags. Create an annotated tag:

```sh
git tag -a v0.1.0 -m "v0.1.0: initial release"
git push origin v0.1.0
```

Verify the version is picked up:

```sh
make build
./bin/repoinjector --version
# repoinjector version v0.1.0
```

## 2. Build cross-platform binaries

Go supports cross-compilation natively. Build for all target platforms:

```sh
# Clean previous builds
make clean

# Linux amd64 (most WSL2 and cloud instances)
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X github.com/ezerfernandes/repoinjector/internal/cmd.version=$(git describe --tags --always --dirty)" -o dist/repoinjector-linux-amd64 ./cmd/repoinjector

# Linux arm64 (Raspberry Pi, ARM servers, some WSL2 on ARM)
GOOS=linux GOARCH=arm64 go build -ldflags "-s -w -X github.com/ezerfernandes/repoinjector/internal/cmd.version=$(git describe --tags --always --dirty)" -o dist/repoinjector-linux-arm64 ./cmd/repoinjector

# macOS amd64 (Intel Macs)
GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X github.com/ezerfernandes/repoinjector/internal/cmd.version=$(git describe --tags --always --dirty)" -o dist/repoinjector-darwin-amd64 ./cmd/repoinjector

# macOS arm64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w -X github.com/ezerfernandes/repoinjector/internal/cmd.version=$(git describe --tags --always --dirty)" -o dist/repoinjector-darwin-arm64 ./cmd/repoinjector
```

The flags `-s -w` strip debug info and DWARF symbols to reduce binary size.

## 3. Create a GitHub release

Upload all binaries to a GitHub release using `gh`:

```sh
gh release create v0.1.0 \
  dist/repoinjector-linux-amd64 \
  dist/repoinjector-linux-arm64 \
  dist/repoinjector-darwin-amd64 \
  dist/repoinjector-darwin-arm64 \
  --title "v0.1.0" \
  --notes "Initial release."
```

To generate release notes from commits automatically:

```sh
gh release create v0.1.0 \
  dist/* \
  --title "v0.1.0" \
  --generate-notes
```

## 4. Installing from a release

### On WSL2 / Linux (amd64)

```sh
# Download the binary
curl -Lo repoinjector https://github.com/ezerfernandes/repoinjector/releases/latest/download/repoinjector-linux-amd64

# Make it executable
chmod +x repoinjector

# Move to a directory in your PATH
sudo mv repoinjector /usr/local/bin/

# Verify
repoinjector --version
```

### On Linux ARM

```sh
curl -Lo repoinjector https://github.com/ezerfernandes/repoinjector/releases/latest/download/repoinjector-linux-arm64
chmod +x repoinjector
sudo mv repoinjector /usr/local/bin/
```

### On macOS (Apple Silicon)

```sh
curl -Lo repoinjector https://github.com/ezerfernandes/repoinjector/releases/latest/download/repoinjector-darwin-arm64
chmod +x repoinjector
sudo mv repoinjector /usr/local/bin/
```

## Automating with GitHub Actions (optional)

Create `.github/workflows/release.yml` to build and publish automatically when you push a tag:

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Run tests
        run: make test

      - name: Build binaries
        run: |
          mkdir -p dist
          VERSION=${GITHUB_REF_NAME}
          LDFLAGS="-s -w -X github.com/ezerfernandes/repoinjector/internal/cmd.version=${VERSION}"

          GOOS=linux   GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o dist/repoinjector-linux-amd64  ./cmd/repoinjector
          GOOS=linux   GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o dist/repoinjector-linux-arm64  ./cmd/repoinjector
          GOOS=darwin  GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o dist/repoinjector-darwin-amd64 ./cmd/repoinjector
          GOOS=darwin  GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o dist/repoinjector-darwin-arm64 ./cmd/repoinjector

      - name: Create GitHub release
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          gh release create ${GITHUB_REF_NAME} \
            dist/* \
            --title "${GITHUB_REF_NAME}" \
            --generate-notes
```

With this workflow in place, the full release process becomes:

```sh
git tag -a v0.2.0 -m "v0.2.0: description of changes"
git push origin v0.2.0
# GitHub Actions builds and publishes the release automatically
```

## Quick reference: full manual release

```sh
VERSION=v0.1.0

# Tag and push
git tag -a $VERSION -m "$VERSION"
git push origin $VERSION

# Build all platforms
mkdir -p dist
LDFLAGS="-s -w -X github.com/ezerfernandes/repoinjector/internal/cmd.version=$VERSION"
GOOS=linux  GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/repoinjector-linux-amd64  ./cmd/repoinjector
GOOS=linux  GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/repoinjector-linux-arm64  ./cmd/repoinjector
GOOS=darwin GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/repoinjector-darwin-amd64 ./cmd/repoinjector
GOOS=darwin GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/repoinjector-darwin-arm64 ./cmd/repoinjector

# Publish
gh release create $VERSION dist/* --title "$VERSION" --generate-notes
```
