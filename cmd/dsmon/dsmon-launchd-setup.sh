#!/bin/bash
# dsmon-launchd-setup.sh
# Run once from YOUR terminal (not from KiroCrew) to register the dashboard
# as a persistent macOS LaunchAgent. After this, the server starts automatically
# on login and restarts if it crashes.
#
# Usage: bash cmd/dsmon/dsmon-launchd-setup.sh
#
# The server reads its data from the default state directory (~/.downshift) and
# serves the dashboard embedded in the binary; `downshift serve` takes no
# arguments. Output goes to ~/.downshift/logs/dsmon-server.log (directory 0700).
#
# DSMON_PLIST_ONLY=1 writes the plist and stops before touching launchd (tests).

set -e
umask 077

LABEL="com.tiagovilasboas.dsmon-server"
PLIST_PATH="$HOME/Library/LaunchAgents/$LABEL.plist"
BINARY="$(cd "$(dirname "$0")/../.." && pwd)/downshift"
LOG_DIR="$HOME/.downshift/logs"
LOG_FILE="$LOG_DIR/dsmon-server.log"

# xml_escape makes a path safe inside a <string> element.
xml_escape() {
  printf '%s' "$1" | sed -e 's/&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g'
}

echo "→ Binary:  $BINARY"
echo "→ Log:     $LOG_FILE"
echo "→ Plist:   $PLIST_PATH"

# Check binary exists
if [ ! -f "$BINARY" ]; then
  echo "✗ Binary not found. Run: go build -o downshift ./cmd/downshift"
  exit 1
fi

mkdir -p "$LOG_DIR" "$(dirname "$PLIST_PATH")"
chmod 700 "$HOME/.downshift" "$LOG_DIR"

BINARY_XML="$(xml_escape "$BINARY")"
LOG_XML="$(xml_escape "$LOG_FILE")"

# Write plist (0600 through the umask)
cat > "$PLIST_PATH" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>$LABEL</string>
    <key>ProgramArguments</key>
    <array>
        <string>$BINARY_XML</string>
        <string>serve</string>
    </array>
    <key>KeepAlive</key>
    <true/>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$LOG_XML</string>
    <key>StandardErrorPath</key>
    <string>$LOG_XML</string>
    <key>ThrottleInterval</key>
    <integer>10</integer>
</dict>
</plist>
PLIST
chmod 600 "$PLIST_PATH"

if [ "${DSMON_PLIST_ONLY:-0}" = 1 ]; then
  echo "plist written; DSMON_PLIST_ONLY=1, launchd not touched"
  exit 0
fi

# Unload existing if any, then load
launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true
launchctl bootstrap "gui/$(id -u)" "$PLIST_PATH"

# Verify
sleep 1
if curl -s http://127.0.0.1:7474/health | grep -q '"ok":true'; then
  echo "✓ dsmon-server running at http://localhost:7474"
  echo "  Persists across reboots and KiroCrew context resets."
else
  echo "✗ Server did not respond — check: tail $LOG_FILE"
  exit 1
fi
