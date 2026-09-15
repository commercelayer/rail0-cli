# rail0-cli — build and install.
#
#   make build      build ./rail0 in the repo
#   make install    install `rail0` + the short `r0` alias into PREFIX
#   make uninstall   remove both from PREFIX
#   make test        run the unit tests
#
# PREFIX defaults to /usr/local/bin; override e.g. `make install PREFIX=$HOME/go/bin`.

BIN    ?= rail0
SHORT  ?= r0
PREFIX ?= /usr/local/bin

# The version `rail0 --version` reports. Derived from git, overridable by the
# caller (goreleaser passes the release tag). Without this the binary reports
# "dev" forever — cmd/version.go's default — and there is then NO way to tell a
# freshly built CLI from one seven weeks old. That is not hypothetical: a stale
# /usr/local/bin/rail0 failed to sign a v1.4.0 authorize, and `--version` said
# `dev`, so the cause took a file timestamp and a checksum against a fresh build
# to find.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build install uninstall test

build:
	go build -ldflags "-X github.com/rail0/cli/cmd.version=$(VERSION)" -o $(BIN) .

# Install the binary as `rail0` plus a short `r0` alias (a symlink to it), so
# both names work from the terminal.
install: build
	install -m 0755 $(BIN) $(PREFIX)/$(BIN)
	ln -sf $(BIN) $(PREFIX)/$(SHORT)

uninstall:
	rm -f $(PREFIX)/$(BIN) $(PREFIX)/$(SHORT)

test:
	go test ./...
