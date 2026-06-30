class JiraTui < Formula
  desc "Terminal-first Jira client"
  homepage "https://github.com/jcharette/jira-tui"
  version "1.0.10"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.10/jira-tui_1.0.10_darwin_arm64.tar.gz"
      sha256 "0c79642970531101aa7ea43e3dba1cc66356b6a549e6084fc5e14660517e4e07"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.10/jira-tui_1.0.10_darwin_amd64.tar.gz"
      sha256 "5557c80c9406c2e8fdcff95a9b6bc39a1c80600209fd95cfad87cd6acdd5428d"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.10/jira-tui_1.0.10_linux_arm64.tar.gz"
      sha256 "2e06d9de7453bc42e9a32d2d2136d15a9f2a42aa59b7d9a566b30b6b18f3974e"
    else
      url "https://github.com/jcharette/jira-tui/releases/download/v1.0.10/jira-tui_1.0.10_linux_amd64.tar.gz"
      sha256 "7ef84922449b16d79fad17b74d6149eb8242e563e683c31d9deabc44d715c21c"
    end
  end

  def install
    bin.install "jira"
  end

  test do
    assert_path_exists bin/"jira"
  end
end
