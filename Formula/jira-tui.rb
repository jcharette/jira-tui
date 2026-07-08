class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "1.0.12"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.12/jira-tui_1.0.12_darwin_arm64.tar.gz"
      sha256 "54a62f68bf65ce84842789fb198ffdedf34f68be3a8b5f2387b72c21a42b3a0f"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.12/jira-tui_1.0.12_darwin_amd64.tar.gz"
      sha256 "f5443cfcb4951647682a25f66a9e2a04fc4089a9e559462872acec7971d0cca9"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.12/jira-tui_1.0.12_linux_arm64.tar.gz"
      sha256 "15afbb3d0d482d8d9e47cfd34f8a10420806bea0eb53138f1d0a6966964358d0"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.12/jira-tui_1.0.12_linux_amd64.tar.gz"
      sha256 "1875accb88e5bd64bb90be7fcf087fe8e93be14078219f11074d74997c79c107"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
