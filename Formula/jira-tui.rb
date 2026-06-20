class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "1.0.3"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.3/jira-tui_1.0.3_darwin_arm64.tar.gz"
      sha256 "6aac94ec439f2290de4ce7142b72aafcb7eddf61c91f0e18c25cb95021dccb1e"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.3/jira-tui_1.0.3_darwin_amd64.tar.gz"
      sha256 "c0ba5ed9158271760967d57472ad94e5ec3b8f35e0e9c3167db30022f5cc9598"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.3/jira-tui_1.0.3_linux_arm64.tar.gz"
      sha256 "5bc32ade8d4c2d8ceecd958b2b3a6fe6be2fc049205d6afeb90a329c868c5d20"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.3/jira-tui_1.0.3_linux_amd64.tar.gz"
      sha256 "70dcffc03cb87dfc851e8c47da1780981648955e6bde096ef0969ef017eda378"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
