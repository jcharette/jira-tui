class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "0.2.2"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v0.2.2/jira-tui_0.2.2_darwin_arm64.tar.gz"
      sha256 "69e6009b58b4c57f2400978115afa821378bb8647f14dd4719ab8f63d473adbe"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v0.2.2/jira-tui_0.2.2_darwin_amd64.tar.gz"
      sha256 "9a58032b7f2902d6b3e5b430bac6b106308e75db9d7f5df71a5fd3791f884a78"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v0.2.2/jira-tui_0.2.2_linux_arm64.tar.gz"
      sha256 "36824a86814af945f508fc86f924ebbdf30ab2cc1c232553f43ac33338418534"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v0.2.2/jira-tui_0.2.2_linux_amd64.tar.gz"
      sha256 "937c1d5f359ce7b11b2d390b525a3720d42df20a8db6557a24086882d50e9c80"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
