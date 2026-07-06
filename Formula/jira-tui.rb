class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "1.0.11"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.11/jira-tui_1.0.11_darwin_arm64.tar.gz"
      sha256 "f5620c861d6670ef6ce40a92684118bda76eee00ec285cf89c320b8c8651d259"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.11/jira-tui_1.0.11_darwin_amd64.tar.gz"
      sha256 "1d0f155ab42107f81ebf5449d756f5b590fe107f582dcd66be0100f31b301be5"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.11/jira-tui_1.0.11_linux_arm64.tar.gz"
      sha256 "dc4f42e6cd3f22c485950cc7083478cba243db09c1d0b14a3d6cba90241974a4"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.11/jira-tui_1.0.11_linux_amd64.tar.gz"
      sha256 "85cb2303616c3f3ea70d375011436bf3ac300bd99fb3c216c6e92edaeadaabbf"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
