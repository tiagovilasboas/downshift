#!/bin/bash
# dsmon-launchd-setup.sh
# Run once from YOUR terminal (not from KiroCrew) to register the dashboard
# as a persistent macOS LaunchAgent. After this, the server starts automatically
# on login and restarts if it crashes.
#
# Usage: bash cmd/dsmon/dsmon-launchd-setup.sh

set -e

PLIST_PATH="$HOME/Library/LaunchAgents/com.tiagovilasboas.dsmon-server.plist"
BINARY="$(cd "$(dirname "$0")/../.." && pwd)/downshift"
WEB_DIR="$(cd "$(dirname "$0")/../.." && pwd)/web"

echo "→ Binary:  $BINARY"
echo "→ Web dir: $WEB_DIR"
echo "→ Plist:   $PLIST_PATH"

# Check binary exists
if [ ! -f "$BINARY" ]; then
  echo "✗ Binary not found. Run: go build -o downshift ./cmd/downshift"
  exit 1
fi

# Unload existing if any
launchctl bootout "gui/$(id -u)/com.tiagovilasboas.dsmon-server" 2>/dev/null || true

# Write plist
cat > "$PLIST_PATH" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.tiagovilasboas.dsmon-server</string>
    <key>ProgramArguments</key>
    <array>
        <string>$BINARY</string>
        <string>serve</string>
        <string>--events</string>
        <string>$HOME/.harness-downshift/events.jsonl</string>
        <string>--agents</string>
        <string>$HOME/.harness-downshift/agents.jsonl</string>
        <string>--web</string>
        <string>$WEB_DIR</string>
    </array>
    <key>KeepAlive</key>
    <true/>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/dsmon-server.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/dsmon-server.log</string>
    <key>ThrottleInterval</key>
    <integer>10</integer>
</dict>
</plist>
PLIST

# Load
launchctl bootstrap "gui/$(id -u)" "$PLIST_PATH"

# Verify
sleep 1
if curl -s http://127.0.0.1:7474/health | grep -q '"ok":true'; then
  echo "✓ dsmon-server running at http://localhost:7474"
  echo "  Persists across reboots and KiroCrew context resets."
else
  echo "✗ Server did not respond — check: tail /tmp/dsmon-server.log"
  exit 1
fi
