class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "1.0.4"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.4/jira-tui_1.0.4_darwin_arm64.tar.gz"
      sha256 "86c4215c27c027a017f9988daf6e80a1e05eb408e79de688e140b379fedf472f"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.4/jira-tui_1.0.4_darwin_amd64.tar.gz"
      sha256 "75530d50cbe0cade71fadef2cc5c0d08afdd214fee24575db9bad564f1425de2"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.4/jira-tui_1.0.4_linux_arm64.tar.gz"
      sha256 "61af9db14a9a927f06ae86cdf11ac2e86c525ff69a59fe1b030293c183d5b526"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.4/jira-tui_1.0.4_linux_amd64.tar.gz"
      sha256 "026a18099591ce3ecc2bc4dfd59fae6100123582849ec25dd1a29b7d18ebfe59"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
