# Installation

TokenMeter ships as a single Go binary named `tm`.

The binary contains the CLI, daemon entrypoint, web server, embedded web assets, and maintenance commands.

You do not need Node or VitePress to run TokenMeter.

This page covers install paths for users and contributors.

## Quick Install Script

The README documents a shell installer:

```bash
curl -sL https://raw.githubusercontent.com/tt-a1i/tokenmeter/main/install.sh | sh
```

Use this when you want the project-maintained installer path.

After install, verify:

```bash
tm version
```

Then run:

```bash
tm setup
tm daily
```

## Go Install

If you already have Go 1.24 or newer:

```bash
go install github.com/tt-a1i/tokenmeter/cmd/tm@latest
```

This builds from the module source.

The binary lands in your Go binary directory.

Make sure that directory is on PATH.

Use `tm version --check` to check for newer releases later.

## Source Build

Clone the repository:

```bash
git clone https://github.com/tt-a1i/tokenmeter.git
cd tokenmeter
make build
```

The project binary entrypoint is `cmd/tm`.

The Makefile is the normal local build path.

Use `make install` when you want to copy the binary into your Go binary path.

Run tests before publishing local changes:

```bash
go test ./...
go vet ./...
```

## Platform Release Artifacts

GoReleaser is configured in `.goreleaser.yml`.

It builds `tm` for `darwin`, `linux`, and `windows`.

It targets `amd64` and `arm64`.

Archives are `tar.gz` by default.

Windows archives use `zip`.

Release checksums are written to `checksums.txt`.

The release binary is named `tm`.

Download the archive for your OS and architecture.

Place `tm` somewhere on PATH.

Run `tm version` after extraction.

## Homebrew Cask

GoReleaser also contains a Homebrew cask section.

The configured tap repository is `tt-a1i/homebrew-tap`.

Publishing requires the `HOMEBREW_TAP_GITHUB_TOKEN` secret.

If the token is missing, release automation skips Homebrew upload and still creates binary artifacts.

When the tap is available, the documented command is:

```bash
brew install --cask tt-a1i/tap/tm
```

Treat Homebrew as a release integration, not the only install path.

## After Installation

Register hooks with `tm setup`.

Start collection with `tm daemon` or by using `tm web`.

Print usage with `tm daily`.

Open the dashboard with `tm web`.

Run `tm doctor` when setup does not look right.
