class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "1.0.1"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.1/jira-tui_1.0.1_darwin_arm64.tar.gz"
      sha256 "dea52c677db2958720cb2156168887e7dbd92c30b0dcdc662b618b0c50e095ca"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.1/jira-tui_1.0.1_darwin_amd64.tar.gz"
      sha256 "7815cb4203ad0489d5e9da5884301ece820b35484fd40dfed0bbb92487f6e99d"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.1/jira-tui_1.0.1_linux_arm64.tar.gz"
      sha256 "fbe6cfc06f2f90a72470f17e3e18bafe20d2e812613e6587ff73b604866bf4b0"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.1/jira-tui_1.0.1_linux_amd64.tar.gz"
      sha256 "5e51da215499781b34a2971b9a618e65766a18c809f6654c1e116c89df4f0a60"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
