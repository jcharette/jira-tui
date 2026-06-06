#!/usr/bin/env sh
set -eu

milestone="${1:?usage: milestone-complete.sh M0 [roadmap]}"
roadmap="${2:-docs/roadmap.md}"
today="${TODAY:-$(date +%Y-%m-%d)}"

MILESTONE="$milestone" TODAY="$today" perl -0pi -e '
  my $milestone = $ENV{"MILESTONE"};
  my $today = $ENV{"TODAY"};
  my $pattern = qr/(###\s+\Q$milestone\E:[\s\S]*?Status:\s*)[^\n]+/;
  s/$pattern/$1 . "complete ($today)"/e
    or die "milestone $milestone not found or does not have a Status line\n";
' "$roadmap"

