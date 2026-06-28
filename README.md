# tiny-server: a simple static HTTP server

[![Go](https://github.com/dilipgurung/tiny-server/actions/workflows/build.yml/badge.svg)](https://github.com/dilipgurung/tiny-server/actions/workflows/build.yml?query=branch%3Amain++)
[![Go](https://github.com/dilipgurung/tiny-server/actions/workflows/release.yml/badge.svg)](https://github.com/dilipgurung/tiny-server/actions/workflows/release.yml)

tiny-server is a simple static HTTP server.

It comes with a live page reload feature. It watches for any changes in your root directory and reloads the page when changes are detected.

The watcher reads `.gitignore` from the served directory and applies **simple basename ignore patterns**: only the final path component is matched (no directory paths, no negation with `!`, no `**` globs). Patterns are matched with Go's [`filepath.Match`](https://pkg.go.dev/path/filepath#Match).

## Usage

### Start tiny-server

```
tiny-server
```

To stop the server, press Ctrl + C in the terminal.

### Show help

```
tiny-server -h
```

or

```
tiny-server -help
```

### Options

| Flag | Description                                  |
| ---- | -------------------------------------------- |
| `-p` | Port to listen on (default `8000`).          |
| `-d` | Directory to serve files from (default `./public` if present, else `.`). |
| `-v` | Print version and exit.                      |
| `-h` | Print usage and exit.                         |

Example:

```
tiny-server -p 8000 -d ./public
```

## Installation

### With Go (version 1.24 or higher)

```bash
go install github.com/dilipgurung/tiny-server@latest
```

### With install.sh

```shell
# Install latest
curl -sSfL https://raw.githubusercontent.com/dilipgurung/tiny-server/main/install.sh | sh

# Install specific version (e.g. v0.2.0)
curl -sSfL https://raw.githubusercontent.com/dilipgurung/tiny-server/main/install.sh | sh -s -- --version v0.2.0

tiny-server -v
```

### Development

```shell
# Fork this project

# Clone it
git clone git@github.com:<YOUR USERNAME>/tiny-server.git

# Install dependencies
cd tiny-server
make deps

# Run
make dev
```

### Release

```shell
# Checkout to main
git checkout main

# Make a release and push to github
make release VERSION=v1.0.0

# The CI will process and release the new version
```

Tags are immutable: `make release` requires an explicit `VERSION` and refuses to overwrite an existing tag.

## License

[MIT License](LICENSE)
