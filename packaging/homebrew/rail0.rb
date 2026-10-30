# The homebrew-core formula for rail0 (`brew install rail0`), kept here so it is
# reviewed and tested with the code it builds. It is submitted to Homebrew/homebrew-core
# once the repository meets Homebrew's notability bar; from then on Homebrew's own bots
# bump `url` and `sha256` on each release. Until then it is exercised by
# bin/brew-formula-test, which builds it from the working tree in a throwaway tap.
#
# Built from source, as homebrew-core requires: no prebuilt binary, the rail0-go SDK and
# every other dependency from the Go module proxy. The build is stamped
# selfupdate.Packager=Homebrew, so the binary never replaces itself and never checks
# for updates on its own: `brew upgrade rail0` is the way to update.
class Rail0 < Formula
  desc "Command-line client for the RAIL0 stablecoin payment gateway"
  homepage "https://github.com/commercelayer/rail0-cli"
  # Set to the release being submitted (the first one whose go.mod needs no sibling
  # checkout: after rail0-cli#186).
  url "https://github.com/commercelayer/rail0-cli/archive/refs/tags/v1.8.0.tar.gz"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"
  license "MIT"
  head "https://github.com/commercelayer/rail0-cli.git", branch: "main"

  livecheck do
    url :stable
    strategy :github_latest
  end

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X github.com/rail0/cli/cmd.version=#{version}
      -X github.com/rail0/cli/internal/selfupdate.Packager=Homebrew
    ]
    system "go", "build", *std_go_args(ldflags:)
    bin.install_symlink "rail0" => "r0"
    generate_completions_from_executable(bin/"rail0", "completion")
    man1.install Dir["docs/man/*.1"]
  end

  test do
    # The stamped version, under both names.
    assert_match version.to_s, shell_output("#{bin}/rail0 version")
    assert_match version.to_s, shell_output("#{bin}/r0 version")

    # Offline and deterministic in shape: a fresh key pair, never written to disk.
    ENV["RAIL0_NO_UPDATE_CHECK"] = "1"
    key = JSON.parse(shell_output("#{bin}/rail0 keys generate --json 2>/dev/null"))
    assert_match(/\A0x\h{40}\z/, key["address"])
    assert_match(/\A0x\h{64}\z/, key["private_key"])

    # The command tree is the one this formula built (no keychain, no network).
    assert_match "balance", shell_output("#{bin}/rail0 keys --help")
  end
end
