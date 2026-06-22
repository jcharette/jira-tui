class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "1.0.6"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.6/jira-tui_1.0.6_darwin_arm64.tar.gz"
      sha256 "6e17a0f11d13cd2a8167c7202c096b11ca5075ab1205781743b446568cfaffdd"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.6/jira-tui_1.0.6_darwin_amd64.tar.gz"
      sha256 "de6e1996cf25734cc0689d3f5e47b18f1199906908b45aa13e3cee96a114b6d3"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.6/jira-tui_1.0.6_linux_arm64.tar.gz"
      sha256 "3fea719f55fd1f337eefef137fce3584b3416280314af3b5035db8eadc86350f"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.6/jira-tui_1.0.6_linux_amd64.tar.gz"
      sha256 "73898082f54e816b067387b55d8d545cd2a30f694d38d6556c8acb44b8eb61a7"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
