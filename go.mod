module github.com/rail0/cli

go 1.26.0

require (
	charm.land/lipgloss/v2 v2.0.6
	github.com/charmbracelet/huh v1.0.0
	github.com/charmbracelet/log v1.0.0
	github.com/charmbracelet/x/term v0.2.2
	github.com/commercelayer/rail0-go v1.7.0
	github.com/ethereum/go-ethereum v1.17.7
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/zalando/go-keyring v0.2.8
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/ProjectZKM/Ziren/crates/go-runtime/zkvm_runtime v0.0.0-20251001021608-1fe7b43fc4d6 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.24.6 // indirect
	github.com/catppuccin/go v0.3.0 // indirect
	github.com/charmbracelet/bubbles v1.0.0 // indirect
	github.com/charmbracelet/bubbletea v1.3.10 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/lipgloss v1.1.0 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20260811164956-006e29f97886 // indirect
	github.com/charmbracelet/x/ansi v0.11.8 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.15 // indirect
	github.com/charmbracelet/x/exp/strings v0.1.0 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/consensys/gnark-crypto v0.18.1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.6 // indirect
	github.com/crate-crypto/go-eth-kzg v1.5.0 // indirect
	github.com/danieljoos/wincred v1.2.3 // indirect
	github.com/dchest/uniuri v1.2.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/ethereum/c-kzg-4844/v2 v2.1.8 // indirect
	github.com/go-logfmt/logfmt v0.6.1 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.24 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/relvacode/iso8601 v1.1.1-0.20210511065120-b30b151cc433 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/spruceid/siwe-go v0.2.1 // indirect
	github.com/supranational/blst v0.3.16 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/exp v0.0.0-20260508232706-74f9aab9d74a // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)

// Still the sibling checkout, because rail0-go is still a private repo the module proxy
// cannot fetch. The require above records the real minimum — v1.7.0, the first release
// with SignSigningPayloadFor / PayloadExpectationFor (and VerifyUnsignedTransaction
// requiring `to`), which cmd/tx_verify.go and cmd/signing.go delegate to — so once the repo
// is public this is a one-line change: drop this directive.
replace github.com/commercelayer/rail0-go => ../rail0-go
