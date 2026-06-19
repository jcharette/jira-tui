class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "0.2.1"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v0.2.1/jira-tui_0.2.1_darwin_arm64.tar.gz"
      sha256 "c358a1b9e7627932c393e28ae1450f4d1a9081aeadc502b7fb3e21e69334f220"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v0.2.1/jira-tui_0.2.1_darwin_amd64.tar.gz"
      sha256 "4c54adb6b1c61c2d8c8d21d3db6fe7dbd0aa1dfe5e934aff98e9b62f5c548d38"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v0.2.1/jira-tui_0.2.1_linux_arm64.tar.gz"
      sha256 "8c8e69960e32520cd933d1ab4ce79cc2c4f96c29b56644d577677c59ca5091b0"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v0.2.1/jira-tui_0.2.1_linux_amd64.tar.gz"
      sha256 "6d276d607f2d422aec2ebc85f386bf4cdccb12485ed33a224a3d7a459902d0e6"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
