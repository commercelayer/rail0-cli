module github.com/rail0/cli

go 1.25.0

require (
	github.com/rail0/go-sdk v0.0.0
	github.com/spf13/cobra v1.8.1
)

require (
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/crypto v0.51.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
)

// Replace with a tagged version once rail0-go is published.
replace github.com/rail0/go-sdk => ../rail0-go
