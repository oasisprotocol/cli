# typed: false
# frozen_string_literal: true

class Oasis < Formula
  desc      "CLI for interacting with the Oasis network"
  homepage  "https://github.com/oasisprotocol/cli"
  license   "Apache-2.0"
  version   "0.0.0"         # ──► Automatically bumped on every tag by CI

  # Source tarball for the tagged release
  url "https://github.com/oasisprotocol/cli/archive/refs/tags/v#{version}.tar.gz"
  sha256 "<SHA256_FROM_CI>" # ──► Replaced by release workflow

  depends_on "go" => :build

  livecheck do
    url   :stable
    regex(/^v?(\d+\.\d+\.\d+)$/i)
  end

  def install
    ldflags = %W[
      -s -w
      -X github.com/oasisprotocol/cli/version.Software=#{version}
    ].join(" ")
    system "go", "build", *std_go_args(ldflags: ldflags), "./"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/oasis --version")
  end
end