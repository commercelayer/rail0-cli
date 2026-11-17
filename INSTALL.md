# Installing rail0

`rail0` is a single static binary with no runtime to install. Every
[release](https://github.com/commercelayer/rail0-cli/releases/latest) publishes it for:

| Platform | Intel / AMD | ARM |
|---|---|---|
| macOS | `rail0_<version>_darwin_amd64.tar.gz` | `rail0_<version>_darwin_arm64.tar.gz` (Apple silicon) |
| Linux | `rail0_<version>_linux_amd64.tar.gz` | `rail0_<version>_linux_arm64.tar.gz` |
| Windows | `rail0_<version>_windows_amd64.zip` | `rail0_<version>_windows_arm64.zip` |

Each release also has `checksums.txt`, with the SHA-256 of every archive. Every
archive holds the binary (`rail0`, or `rail0.exe` on Windows) at its root, plus the
shell completions (`completions/`), the man pages (`man/man1/`) and the command
reference (`docs/`).

- [macOS](#macos)
- [Linux](#linux)
- [Windows](#windows)
- [Checking the install](#checking-the-install)
- [Updating](#updating)
- [Uninstalling](#uninstalling)

## macOS

### Install script (recommended)

```sh
curl -fsSL https://raw.githubusercontent.com/commercelayer/rail0-cli/main/install.sh | sh
```

The script:

1. picks the archive for your Mac (Apple silicon or Intel);
2. verifies it against `checksums.txt`, and installs nothing if it does not match;
3. installs `rail0` and the short alias `r0` into `~/.local/bin`, a directory you own,
   so neither the install nor a later `rail0 update` needs `sudo`;
4. prints the line to add if `~/.local/bin` is not on your `PATH`.

On the default zsh that line is usually:

```sh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
```

Optional variables:

- `RAIL0_VERSION=1.9.1` installs a given release.
- `RAIL0_INSTALL_DIR=/somewhere/bin` changes the destination.

### Manual download

1. Download `rail0_<version>_darwin_arm64.tar.gz` (Apple silicon) or `…_darwin_amd64.tar.gz`
   (Intel) and `checksums.txt` from the
   [latest release](https://github.com/commercelayer/rail0-cli/releases/latest). Not
   sure which Mac you have? `uname -m` prints `arm64` on Apple silicon and `x86_64` on Intel.
2. Verify, unpack and install:

   ```sh
   shasum -a 256 -c checksums.txt --ignore-missing
   tar -xzf rail0_*_darwin_*.tar.gz rail0
   mkdir -p ~/.local/bin && mv rail0 ~/.local/bin/ && ln -sf rail0 ~/.local/bin/r0
   ```

3. A file downloaded with a browser is quarantined by macOS, and the binary is not
   notarized by Apple, so the first run is refused ("cannot be opened because the
   developer cannot be verified"). Clear the flag once:

   ```sh
   xattr -d com.apple.quarantine ~/.local/bin/rail0
   ```

   The install script and `rail0 update` download with `curl`, which sets no
   quarantine, so this step is only for browser downloads.

## Linux

### Install script (recommended)

```sh
curl -fsSL https://raw.githubusercontent.com/commercelayer/rail0-cli/main/install.sh | sh
```

It does what it does on macOS (see [above](#install-script-recommended)): it picks
`amd64` or `arm64`, verifies the checksum, and installs `rail0` and `r0` into
`~/.local/bin`. It needs only `sh`, `curl`, `tar` and `sha256sum`, so it also runs in
minimal containers:

```dockerfile
RUN curl -fsSL https://raw.githubusercontent.com/commercelayer/rail0-cli/main/install.sh \
      | RAIL0_INSTALL_DIR=/usr/local/bin sh
```

The binary is static (no glibc needed), so it runs on Alpine as well.

### Manual download

```sh
arch=$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')
version=1.9.1   # the release to install
base=https://github.com/commercelayer/rail0-cli/releases/download/v$version
curl -fsSLO "$base/rail0_${version}_linux_${arch}.tar.gz"
curl -fsSLO "$base/checksums.txt"
sha256sum -c checksums.txt --ignore-missing
tar -xzf "rail0_${version}_linux_${arch}.tar.gz" rail0
mkdir -p ~/.local/bin && mv rail0 ~/.local/bin/ && ln -sf rail0 ~/.local/bin/r0
```

## Windows

There is no install script for Windows. Download the `.zip` yourself, or let PowerShell
do it.

### PowerShell

This installs the latest release for your CPU (x64 or ARM64) into
`%LOCALAPPDATA%\Programs\rail0` and adds that folder to your user `Path`. It needs no
administrator rights.

```powershell
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"   # Windows PowerShell 5.1 downloads far slower with a progress bar
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$repo = "commercelayer/rail0-cli"
$tag  = (Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest").tag_name
$ver  = $tag.TrimStart("v")
$cpu  = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
$arch = if ("$cpu" -eq "Arm64") { "arm64" } else { "amd64" }
$zip  = "rail0_${ver}_windows_${arch}.zip"
$base = "https://github.com/$repo/releases/download/$tag"
$tmp  = Join-Path $env:TEMP $zip
$sums = Join-Path $env:TEMP "rail0_checksums.txt"
Invoke-WebRequest "$base/$zip" -OutFile $tmp -UseBasicParsing
Invoke-WebRequest "$base/checksums.txt" -OutFile $sums -UseBasicParsing

# Install nothing unless the archive matches the release's checksum.
$line = Get-Content $sums | Where-Object { $_ -match "\s$([regex]::Escape($zip))$" }
$want = ($line -split "\s+")[0]
$got  = (Get-FileHash $tmp -Algorithm SHA256).Hash.ToLower()
if (-not $want -or $got -ne $want) { throw "Checksum mismatch for $zip" }

$dir = Join-Path $env:LOCALAPPDATA "Programs\rail0"
Expand-Archive $tmp -DestinationPath $dir -Force
Remove-Item $tmp, $sums

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (($userPath -split ";") -notcontains $dir) {
  [Environment]::SetEnvironmentVariable("Path", "$userPath;$dir", "User")
  $env:Path += ";$dir"
}
rail0 version
```

Open a new terminal afterwards so that every window sees the new `Path`.

### Manual download

1. Download `rail0_<version>_windows_amd64.zip` (or `…_windows_arm64.zip` on an ARM PC
   such as a Snapdragon laptop) and `checksums.txt` from the
   [latest release](https://github.com/commercelayer/rail0-cli/releases/latest).
   **Settings › System › About › System type** says which one you have.
2. Check the archive: `Get-FileHash .\rail0_<version>_windows_amd64.zip` should print
   the same SHA-256 as its line in `checksums.txt`, ignoring case.
3. Extract it into a folder of your own, for example `%LOCALAPPDATA%\Programs\rail0`.
4. Add that folder to your user `Path`: **Settings › System › About › Advanced system
   settings › Environment Variables**, select `Path` under *User variables*, then
   **Edit › New**.

Windows may show a SmartScreen warning the first time a downloaded program runs. Choose
**More info › Run anyway**.

The short alias `r0` is not created on Windows. Use `rail0`, or add a PowerShell alias
to your profile: `Set-Alias r0 rail0`.

## Checking the install

```sh
rail0 version
```

It should print the release you installed. Then continue with `rail0 init`, the guided
setup: see the [README](README.md#guided-setup-rail0-init).

**Shell completions** come with every archive, and the binary can also print them
(`rail0 completion --help`):

```sh
rail0 completion zsh  > "${fpath[1]}/_rail0"     # zsh
rail0 completion bash > ~/.local/share/bash-completion/completions/rail0
rail0 completion fish > ~/.config/fish/completions/rail0.fish
rail0 completion powershell | Out-String | Invoke-Expression   # PowerShell, this session
```

## Updating

Once installed, `rail0` updates itself on every platform, so you never download a
package by hand again:

```sh
rail0 update           # install the latest release in place
rail0 update --check   # only say whether a newer one exists
```

It downloads the archive for your platform, checks its SHA-256 against
`checksums.txt`, and swaps the binary. If the update is interrupted, the old binary is
left as it was. On Windows the running `rail0.exe` is renamed to `rail0.exe.old` and
removed the next time `rail0` runs.

Interactive sessions also check once a day and ask `Update now? [Y/n]`. That check never
runs in CI, with `--json`/`--quiet`, or with `RAIL0_NO_UPDATE_CHECK=1`.

If `rail0` lives in a folder you cannot write to (e.g. `/usr/local/bin`), run
`sudo rail0 update`. The default install folders (`~/.local/bin`,
`%LOCALAPPDATA%\Programs\rail0`) need no extra rights.

## Uninstalling

Signing keys saved with `rail0 keys add` or `rail0 keys generate` live in the OS keychain
(macOS Keychain, Windows Credential Manager, the Secret Service on Linux), not next to
the binary. Remove each one with `rail0 keys rm <name>` **before** deleting `rail0`, if
you no longer want it stored, and make sure you hold a copy of any key that still
controls funds.

- **macOS / Linux:** `rm ~/.local/bin/rail0 ~/.local/bin/r0`. The configuration (`rail0
  config path` prints where) and the session tokens in the user cache directory can be
  removed too.
- **Windows:** delete `%LOCALAPPDATA%\Programs\rail0` and remove it from your user
  `Path`.
