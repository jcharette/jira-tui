class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "1.0.7"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.7/jira-tui_1.0.7_darwin_arm64.tar.gz"
      sha256 "e0fcb1700e688eb97fd40cf0f8afdd1f25fadfb41585936b09dca5700bdced31"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.7/jira-tui_1.0.7_darwin_amd64.tar.gz"
      sha256 "e297e526a07e403758b2bb7c3c08bdf7d54b90b766002023649be3f76cc9a25a"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.7/jira-tui_1.0.7_linux_arm64.tar.gz"
      sha256 "ac46bfb8bf461d36cb592e4ce263e72b0e76000a74d379e712f29d8867731504"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.7/jira-tui_1.0.7_linux_amd64.tar.gz"
      sha256 "d29c969b3ae54990dec5c17448ead214bbde694ff3d32e93a157e336ac00fe39"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
