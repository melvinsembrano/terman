class Terman < Formula
  desc "Terminal API Client: TUI and CLI for building and running HTTP requests"
  homepage "https://github.com/melvinsembrano/terman"
  version "0.6.0"
  license "MIT"

  on_macos do
    on_intel do
      url "https://github.com/melvinsembrano/terman/releases/download/v#{version}/terman-#{version}-darwin-amd64.tar.gz"
      sha256 "PLACEHOLDER_DARWIN_AMD64"
    end
    on_arm do
      url "https://github.com/melvinsembrano/terman/releases/download/v#{version}/terman-#{version}-darwin-arm64.tar.gz"
      sha256 "PLACEHOLDER_DARWIN_ARM64"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/melvinsembrano/terman/releases/download/v#{version}/terman-#{version}-linux-amd64.tar.gz"
      sha256 "PLACEHOLDER_LINUX_AMD64"
    end
    on_arm do
      url "https://github.com/melvinsembrano/terman/releases/download/v#{version}/terman-#{version}-linux-arm64.tar.gz"
      sha256 "PLACEHOLDER_LINUX_ARM64"
    end
  end

  def install
    bin.install "terman"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/terman version")
  end
end
