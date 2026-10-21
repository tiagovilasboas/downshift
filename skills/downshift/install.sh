#!/bin/bash
# Install downshift skill to common harness skill directories
set -e

SKILL_NAME="downshift"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SKILL_FILE="$SCRIPT_DIR/SKILL.md"

install_to() {
    local dir="$1"
    if [ -d "$(dirname "$dir")" ]; then
        mkdir -p "$dir"
        cp "$SKILL_FILE" "$dir/SKILL.md"
        echo "✓ Installed to $dir"
    fi
}

# KiroCrew
install_to "$HOME/.kiro/crew/skills/$SKILL_NAME"

# Kiro IDE
install_to "$HOME/.kiro/skills/$SKILL_NAME"

# Claude Code (if skills directory exists)
if [ -d "$HOME/.claude" ]; then
    install_to "$HOME/.claude/skills/$SKILL_NAME"
fi

# Cursor
if [ -d "$HOME/.cursor" ]; then
    install_to "$HOME/.cursor/skills/$SKILL_NAME"
fi

echo ""
echo "Skill '$SKILL_NAME' installed. The agent will auto-load it when running shell commands."
