# Install

## Homebrew

Install directly from this repo's formula:

```bash
brew install --formula https://raw.githubusercontent.com/jcharette/jira-tui/main/Formula/jira-tui.rb
```

To publish a dedicated tap later, copy `Formula/jira-tui.rb` into a repository named
`homebrew-jira-tui`; then users can install with:

```bash
brew install jcharette/jira-tui/jira-tui
```

For local testing from a checkout:

```bash
brew install --build-from-source ./Formula/jira-tui.rb
```

The formula installs the `jira` binary.

## Release Binary

Download a release archive from GitHub Releases, unpack it, and move `jira` onto your `PATH`.

Apple Silicon example:

```bash
curl -LO https://github.com/jcharette/jira-tui/releases/download/v1.0.4/jira-tui_1.0.4_darwin_arm64.tar.gz
tar -xzf jira-tui_1.0.4_darwin_arm64.tar.gz
install -m 0755 jira ~/bin/jira
```

## Go Install

```bash
go install github.com/jcharette/jira-tui/cmd/jira@v1.0.4
```

Go installs the binary as `jira`.

## Source Checkout

```bash
go mod download
make install-user
```
