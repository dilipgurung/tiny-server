# tiny-server: a simple static HTTP server

tiny-server is a simple static HTTP server.

It comes with a live page reload feature. It watches for any changes in your root directory and reloads the page when changes are detected.

## Usage

### Start tiny-server

```
tiny-sever
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

## Installation

### With Go (version 1.22 or higher)

```bash
go install github.com/dilipgurung/tiny-server@latest
```

### With bash

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

# Run the program
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

## License

[MIT License](LICENSE)
