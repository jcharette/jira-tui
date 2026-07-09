class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "1.0.15"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.15/jira-tui_1.0.15_darwin_arm64.tar.gz"
      sha256 "c5838aaa928ba661f14510d04703daa1bd9f024b9a62eeeccb117b1239609eab"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.15/jira-tui_1.0.15_darwin_amd64.tar.gz"
      sha256 "eb7d1b48eb07d8d5961bef2dce4f62bb149eedfaf80e0ed8255c68c7a00d86a0"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.15/jira-tui_1.0.15_linux_arm64.tar.gz"
      sha256 "3c4b67ba91f1ce426e00abba01efacdabc0ebee5d6c6bfa2280ffc91c60bc15a"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.15/jira-tui_1.0.15_linux_amd64.tar.gz"
      sha256 "4fb8b61b34800745d16761953b225f48d8434a82d67bfa9a9bb06b7ab8faea98"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
