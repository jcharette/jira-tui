#!/usr/bin/env sh
set -eu

version="${1:?usage: release.sh VERSION [changelog]}"
changelog="${2:-docs/releases/CHANGELOG.md}"
today="${TODAY:-$(date +%Y-%m-%d)}"

VERSION="$version" TODAY="$today" perl -0pi -e '
  my $version = $ENV{"VERSION"};
  my $today = $ENV{"TODAY"};
  die "release $version already exists\n" if /^##\s+\Q$version\E\s+-/m;
  s/(## Unreleased\n\n)/$1 . "## $version - $today\n\n"/e
    or die "Unreleased section not found\n";
' "$changelog"

