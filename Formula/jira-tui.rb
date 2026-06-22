class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "1.0.8"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.8/jira-tui_1.0.8_darwin_arm64.tar.gz"
      sha256 "c09df533b437f25613bf426309fedcf9d76e752491b0759203e251d4a1ea6594"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.8/jira-tui_1.0.8_darwin_amd64.tar.gz"
      sha256 "57e413fd80b0f1c6ee60b2562d8ad604710caa95d69ea5e2b52a12e3e5551876"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.8/jira-tui_1.0.8_linux_arm64.tar.gz"
      sha256 "d468938e85c49194c73b492b782a60839cfa9a2e15f4aa813857d11b241d31c8"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.8/jira-tui_1.0.8_linux_amd64.tar.gz"
      sha256 "5251d7f32ec10f1f20c9f730cbd82dbc87f12d0a68d9fe6ab9ed8dd046547dbd"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
