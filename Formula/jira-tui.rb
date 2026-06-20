class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "1.0.2"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.2/jira-tui_1.0.2_darwin_arm64.tar.gz"
      sha256 "a867e3d36cab7a0dc2306a7d8575eda3845dc1afb5776567757ef2220dcea708"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.2/jira-tui_1.0.2_darwin_amd64.tar.gz"
      sha256 "55bdeb5e88c87d5b95e4fa3c5c6bbdd72935e892d0baf9f1ae81ed765fbcaa72"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.2/jira-tui_1.0.2_linux_arm64.tar.gz"
      sha256 "14e4648194857e9e5faab051c6d041cc8ee62d8ff0c0871eb88cd508b0724152"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.2/jira-tui_1.0.2_linux_amd64.tar.gz"
      sha256 "50562473842cc6ff59893c5c02ae004e513b575bd27d7008848c8a18bef6a36e"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
