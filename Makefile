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

.PHONY: build install uninstall test

build:
	go build -o $(BIN) .

# Install the binary as `rail0` plus a short `r0` alias (a symlink to it), so
# both names work from the terminal.
install: build
	install -m 0755 $(BIN) $(PREFIX)/$(BIN)
	ln -sf $(BIN) $(PREFIX)/$(SHORT)

uninstall:
	rm -f $(PREFIX)/$(BIN) $(PREFIX)/$(SHORT)

test:
	go test ./...
