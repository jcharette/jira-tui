class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "1.0.14"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.14/jira-tui_1.0.14_darwin_arm64.tar.gz"
      sha256 "c4b2bb5d00a3581f39a7f58ca7c6058f5139d720457666dbc3f3a3184ebcd1cb"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.14/jira-tui_1.0.14_darwin_amd64.tar.gz"
      sha256 "8f3c3645b25e869b9ee859714fb137425d0193462a386042f935fd0617dd7b25"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.14/jira-tui_1.0.14_linux_arm64.tar.gz"
      sha256 "53cdaa6595d4bbaee760befb2f4762b0fb629b450f73bf92c0a1c94e10abd00a"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.14/jira-tui_1.0.14_linux_amd64.tar.gz"
      sha256 "06de1dd03eeed8c412ef65a7d7fac65038c0a9d4b370eb9f106386b681bc9cbe"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
