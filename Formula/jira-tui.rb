class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "1.0.9"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.9/jira-tui_1.0.9_darwin_arm64.tar.gz"
      sha256 "a41e6fd2d3ca6e830dd3a5de261d8da2874b55953a05eb9a5a3637cb9c3eb9f9"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.9/jira-tui_1.0.9_darwin_amd64.tar.gz"
      sha256 "8e37f3cc9eea7de497de705f22a863a8ea6f05709794a63f8fc638e38531d8e4"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.9/jira-tui_1.0.9_linux_arm64.tar.gz"
      sha256 "b0bcfcb6b03336fe696ba3f51faa6c65fbc66ff108d3d536441c5948dd1995eb"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.9/jira-tui_1.0.9_linux_amd64.tar.gz"
      sha256 "b2a287f711fd39d9544ea8cd3e7e48f2944a79b9c39c8287eed24af94557598a"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
