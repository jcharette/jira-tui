class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "1.0.0"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.0/jira-tui_1.0.0_darwin_arm64.tar.gz"
      sha256 "e1470af23bd829263ba8506c40d2760855821d1bb3de25add9e552f50445eecd"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.0/jira-tui_1.0.0_darwin_amd64.tar.gz"
      sha256 "0a878bac735dca7521085110e674cfaa6be8c0b130f313ed4d723080771349dc"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.0/jira-tui_1.0.0_linux_arm64.tar.gz"
      sha256 "93e6765c3ecbc170dcdacb8b51053754c7839e1272e851dfb6bfec44b7c53cea"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.0/jira-tui_1.0.0_linux_amd64.tar.gz"
      sha256 "db8ebc1077accdbfb926da517bbfb1727d2398445de8743b685a4ddc1c125882"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
