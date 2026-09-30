#!/usr/bin/env bash
# launch.sh — abre o dsmon numa janela flutuante/sempre-no-topo
# Detecta automaticamente o terminal disponível.
# Uso: ./cmd/dsmon/launch.sh
set -e

REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
BINARY="$REPO_DIR/dsmon"

# Build se necessário
if [ ! -f "$BINARY" ] || [ "$REPO_DIR/cmd/dsmon/main.go" -nt "$BINARY" ]; then
    echo "building dsmon..."
    cd "$REPO_DIR" && go build -o dsmon ./cmd/dsmon
fi

# Geometria padrão (pixels)
W=620; H=440; X=1200; Y=60

# ── iTerm2 ────────────────────────────────────────────────────────────────────
if pgrep -x "iTerm2" > /dev/null 2>&1; then
    osascript <<APPLESCRIPT
tell application "iTerm2"
    set newWin to (create window with default profile)
    tell current session of newWin
        set name to "dsmon"
        set columns to 62
        set rows to 22
        write text "${BINARY}"
    end tell
    tell newWin
        set bounds to {${X}, ${Y}, $((X+W)), $((Y+H))}
    end tell
    activate
end tell
APPLESCRIPT
    exit 0
fi

# ── kitty ─────────────────────────────────────────────────────────────────────
if command -v kitty > /dev/null 2>&1; then
    kitty \
        --title "dsmon" \
        --class "dsmon" \
        --override "initial_window_width=${W}px" \
        --override "initial_window_height=${H}px" \
        --override "remember_window_size=no" \
        --override "window_margin_width=4" \
        -- "$BINARY" &
    exit 0
fi

# ── Ghostty ───────────────────────────────────────────────────────────────────
if command -v ghostty > /dev/null 2>&1; then
    ghostty \
        --title="dsmon" \
        --initial-command="$BINARY" &
    exit 0
fi

# ── Terminal.app (fallback) ───────────────────────────────────────────────────
osascript <<APPLESCRIPT
tell application "Terminal"
    set newWin to do script "${BINARY}"
    delay 0.3
    set bounds of front window to {${X}, ${Y}, $((X+W)), $((Y+H))}
    activate
end tell
APPLESCRIPT
