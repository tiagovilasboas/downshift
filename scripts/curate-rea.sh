#!/usr/bin/env bash
# Downshift REA Curation Script
# Move reverse engineering content from Downshift to downshift-labs
#
# Usage:
#   ./curate-rea.sh --dry-run   # Preview changes
#   ./curate-rea.sh --apply     # Execute moves
#
# Downshift by Tiago Vilas Boas
# https://github.com/tiagovilasboas/downshift

set -euo pipefail

DOWNSHIFT_DIR="/Users/tiago.boas/Github/downshift"
LABS_DIR="/Users/tiago.boas/Github/downshift-labs"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

DRY_RUN=true

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --apply)
            DRY_RUN=false
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        *)
            echo "Usage: $0 [--dry-run|--apply]"
            exit 1
            ;;
    esac
done

echo -e "${BLUE}=== Downshift REA Curation ===${NC}"
echo "Mode: $([ "$DRY_RUN" = true ] && echo 'DRY RUN (preview)' || echo 'APPLY (execute)')"
echo ""

# Check directories exist
if [[ ! -d "$DOWNSHIFT_DIR" ]]; then
    echo -e "${RED}Error: Downshift directory not found: $DOWNSHIFT_DIR${NC}"
    exit 1
fi

if [[ ! -d "$LABS_DIR" ]]; then
    echo -e "${RED}Error: Labs directory not found: $LABS_DIR${NC}"
    exit 1
fi

echo -e "${YELLOW}Scanning evidence directory...${NC}"
echo ""

# Function to check if file is REA (should move to labs)
is_rea_file() {
    local filename="$1"
    case "$filename" in
        *-runtime-source.md) return 0 ;;
        *-gap-diagnostic.md) return 0 ;;
        *-native-hook-*.md) return 0 ;;
        *-native-hook-*.json) return 0 ;;
        *) return 1 ;;
    esac
}

# Function to check if file is integration (stays in Downshift)
is_integration_file() {
    local filename="$1"
    case "$filename" in
        *-rewrite-honored-*.md) return 0 ;;
        *-executor-ack.*) return 0 ;;
        benchmark-*.md) return 0 ;;
        multilang-*.md) return 0 ;;
        *) return 1 ;;
    esac
}

# Function to detect harness from filename
detect_harness() {
    local filename="$1"
    for harness in kirocrew claude-code codex cursor antigravity opencode; do
        if [[ "$filename" == *"$harness"* ]]; then
            echo "$harness"
            return 0
        fi
    done
    echo ""
}

# Scan evidence directory
echo -e "${GREEN}Files to KEEP in Downshift (integration evidence):${NC}"
for file in "$DOWNSHIFT_DIR"/docs/evidence/*; do
    [[ -f "$file" ]] || continue
    filename=$(basename "$file")
    if is_integration_file "$filename"; then
        echo "  ✓ $filename"
    fi
done
echo ""

echo -e "${YELLOW}Files to MOVE to labs (REA):${NC}"
rea_count=0
for file in "$DOWNSHIFT_DIR"/docs/evidence/*; do
    [[ -f "$file" ]] || continue
    filename=$(basename "$file")
    
    if is_rea_file "$filename"; then
        harness=$(detect_harness "$filename")
        if [[ -n "$harness" ]]; then
            target_dir="$LABS_DIR/reverse/$harness/findings"
            echo "  → $filename"
            echo "    └─ $target_dir/"
            rea_count=$((rea_count + 1))
        else
            echo "  ? $filename (unknown harness)"
        fi
    fi
done

if [[ $rea_count -eq 0 ]]; then
    echo "  (none found)"
fi
echo ""

# Check for spike duplicates
echo -e "${YELLOW}Checking spike duplicates...${NC}"
SPIKE_FILE="$DOWNSHIFT_DIR/docs/spikes/kirocrew-harness-detection.md"
LABS_SPIKE="$LABS_DIR/reverse/kirocrew/SPIKE.md"

if [[ -f "$SPIKE_FILE" ]] && [[ -f "$LABS_SPIKE" ]]; then
    echo "  ⚠ kirocrew-harness-detection.md may duplicate labs SPIKE.md"
    echo "    Downshift: $(wc -l < "$SPIKE_FILE" | tr -d ' ') lines"
    echo "    Labs:      $(wc -l < "$LABS_SPIKE" | tr -d ' ') lines"
    echo "    Recommendation: Consolidate into labs SPIKE, remove from Downshift"
fi
echo ""

# Execute moves if not dry run
if [[ "$DRY_RUN" = false ]] && [[ $rea_count -gt 0 ]]; then
    echo -e "${BLUE}Executing moves...${NC}"
    
    for file in "$DOWNSHIFT_DIR"/docs/evidence/*; do
        [[ -f "$file" ]] || continue
        filename=$(basename "$file")
        
        if is_rea_file "$filename"; then
            harness=$(detect_harness "$filename")
            
            if [[ -n "$harness" ]]; then
                target_dir="$LABS_DIR/reverse/$harness/findings"
                mkdir -p "$target_dir"
                
                # Generate finding number
                existing_count=$(find "$target_dir" -maxdepth 1 -name 'F*.md' 2>/dev/null | wc -l | tr -d ' ')
                next_num=$((existing_count + 1))
                
                # Create finding filename (strip date suffix, add F number)
                base_name=$(echo "$filename" | sed 's/-[0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\}//')
                finding_name="F${next_num}-${base_name}"
                
                # Copy file
                cp "$file" "$target_dir/$finding_name"
                echo -e "  ${GREEN}✓${NC} Copied to $target_dir/$finding_name"
                
                # Remove from Downshift
                rm "$file"
                echo -e "  ${GREEN}✓${NC} Removed $file"
            fi
        fi
    done
    
    echo ""
    echo -e "${GREEN}Done! Remember to:${NC}"
    echo "  1. cd $LABS_DIR && git add -A && git commit -m 'docs(rea): migrate findings from downshift'"
    echo "  2. cd $DOWNSHIFT_DIR && git add -A && git commit -m 'chore: move REA content to labs'"
else
    if [[ $rea_count -gt 0 ]]; then
        echo -e "${BLUE}Run with --apply to execute moves${NC}"
    else
        echo -e "${GREEN}No REA files to move — Downshift is clean!${NC}"
    fi
fi
