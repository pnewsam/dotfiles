#!/bin/bash
# Install modern CLI replacements via Homebrew.
# Safe to re-run — Homebrew skips already-installed packages.

set -e

echo "Installing Homebrew..."
if ! command -v brew &>/dev/null; then
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
fi

echo ""
echo "Installing CLI packages..."

# ── Must: pure upgrades, zero downsides ─────────────────────────

brew install ripgrep       # rg — faster grep, gitignore-aware
brew install fd             # faster find, gitignore-aware
brew install zoxide         # z — smarter cd, learns your habits

# ── Should: better output, visual polish ────────────────────────

brew install bat            # syntax-highlighted cat
brew install eza            # nicer ls with git status icons
brew install git-delta      # syntax-highlighted diffs

# ── Nice: situationally useful ──────────────────────────────────

brew install tealdeer      # tldr — practical examples, no man pages
brew install dust           # smarter du
brew install bottom         # btm — visual process monitor

echo ""
echo "Done. Available commands: rg, fd, z (or zoxide), bat, eza, delta, tldr, dust, btm"