#!/bin/bash
# macOS defaults — run once on a fresh machine.
# Many settings need the app to restart (script handles that at the end).

set -e

echo "Setting macOS defaults..."

# ── Keyboard ──────────────────────────────────────────────────────

# Max key repeat speed (2 is fastest, 1 is off)
defaults write -g KeyRepeat -int 2

# Shortest delay before key repeat (15 is shortest)
defaults write -g InitialKeyRepeat -int 15

# Disable press-and-hold for accented characters (enables key repeat for all keys)
# defaults write -g ApplePressAndHoldEnabled -bool false

# ── Finder ────────────────────────────────────────────────────────

# Show all filename extensions
defaults write com.apple.finder AppleShowAllExtensions -bool true

# Show path bar at bottom
defaults write com.apple.finder ShowPathbar -bool true

# Show status bar
defaults write com.apple.finder ShowStatusBar -bool true

# Show hidden files
defaults write com.apple.finder AppleShowAllFiles -bool true

# Search the current folder by default
defaults write com.apple.finder FXDefaultSearchScope -string "SCcf"

# Don't create .DS_Store on network volumes
defaults write com.apple.desktopservices DSDontWriteNetworkStores -bool true

# Don't create .DS_Store on USB volumes
defaults write com.apple.desktopservices DSDontWriteUSBStores -bool true

# ── Dock ──────────────────────────────────────────────────────────

# Auto-hide
defaults write com.apple.dock autohide -bool true

# Remove auto-hide delay
defaults write com.apple.dock autohide-delay -float 0

# Minimize windows into their app icon
defaults write com.apple.dock minimize-to-application -bool true

# Don't show recent apps
defaults write com.apple.dock show-recents -bool false

# ── Screenshots ───────────────────────────────────────────────────

# Save to ~/Screenshots instead of Desktop
defaults write com.apple.screencapture location -string "$HOME/Screenshots"

# Disable window shadow in screenshots
defaults write com.apple.screencapture disable-shadow -bool true

# ── Save dialogs ──────────────────────────────────────────────────

# Expand save and print panels by default
defaults write -g NSNavPanelExpandedStateForSaveMode -bool true
defaults write -g NSNavPanelExpandedStateForSaveMode2 -bool true
defaults write -g PMPrintingExpandedStateForPrint -bool true
defaults write -g PMPrintingExpandedStateForPrint2 -bool true

# ── Trackpad ──────────────────────────────────────────────────────

# Tap to click
defaults write com.apple.driver.AppleBluetoothMultitouch.trackpad Clicking -bool true
defaults -currentHost write -g com.apple.mouse.tapBehavior -int 1

# ── TextEdit ──────────────────────────────────────────────────────

# Use plain text as default
defaults write com.apple.TextEdit RichText -bool false

# ── Restart affected apps ─────────────────────────────────────────

echo ""
echo "Restarting affected apps..."

for app in Finder Dock SystemUIServer; do
    killall "$app" &>/dev/null || true
done

echo "Done. Some changes require a logout to fully take effect."