class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "1.0.5"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.5/jira-tui_1.0.5_darwin_arm64.tar.gz"
      sha256 "862b510b91bd8a3d177432c661598673ff05442a0b89b367905ff3749e04fa09"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.5/jira-tui_1.0.5_darwin_amd64.tar.gz"
      sha256 "0b821a388e7aed91aaefc1b260b8f2c41a100827b2f41bc80cfccfd7aa8a0af6"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.5/jira-tui_1.0.5_linux_arm64.tar.gz"
      sha256 "1181e00d6fca24d22e1eb78d86b0cf8612ab802c1be4dfd4c047b68ece64e4ae"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.5/jira-tui_1.0.5_linux_amd64.tar.gz"
      sha256 "60704ba2682db47992c069436517c3bcab4e0195af8b2283d774a1adac1766cb"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
