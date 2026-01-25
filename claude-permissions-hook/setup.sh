#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_DIR="${HOME}/.config"
CONFIG_FILE="${CONFIG_DIR}/claude-permissions.toml"
SETTINGS_FILE="${HOME}/.claude/settings.json"
BINARY_PATH="${SCRIPT_DIR}/claude-permissions-hook"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "Claude Permissions Hook Setup"
echo "=============================="
echo

# 1. Build the binary
echo -e "${YELLOW}Building binary...${NC}"
cd "$SCRIPT_DIR"
go build -o claude-permissions-hook .
echo -e "${GREEN}✓ Built ${BINARY_PATH}${NC}"
echo

# 2. Look for user config (my-config.toml, gitignored)
USER_CONFIG="${SCRIPT_DIR}/my-config.toml"

if [[ ! -f "$USER_CONFIG" ]]; then
    if [[ -f "$CONFIG_FILE" ]]; then
        echo -e "${YELLOW}No my-config.toml found, keeping existing ${CONFIG_FILE}${NC}"
        USER_CONFIG=""
    else
        echo -e "${YELLOW}No my-config.toml found, using default config${NC}"
        USER_CONFIG="${SCRIPT_DIR}/default-config.toml"
    fi
fi

# 3. Copy config to ~/.config/
mkdir -p "$CONFIG_DIR"
if [[ -n "$USER_CONFIG" ]]; then
    cp "$USER_CONFIG" "$CONFIG_FILE"
    echo -e "${GREEN}✓ Copied $(basename "$USER_CONFIG") to ${CONFIG_FILE}${NC}"
fi
echo

# 4. Install hook into Claude Code settings
echo -e "${YELLOW}Installing hook...${NC}"

mkdir -p "$(dirname "$SETTINGS_FILE")"

HOOK_COMMAND="${BINARY_PATH} run --config ${CONFIG_FILE}"
MATCHER="Bash|Read|Write|Edit|Skill"

# Create the new hook entry
NEW_HOOK=$(cat <<EOF
{
  "matcher": "${MATCHER}",
  "hooks": [
    {
      "type": "command",
      "command": "${HOOK_COMMAND}"
    }
  ]
}
EOF
)

if [[ -f "$SETTINGS_FILE" ]]; then
    # Check if jq is available
    if ! command -v jq &> /dev/null; then
        echo -e "${RED}Error: jq is required but not installed.${NC}"
        echo "Install with: sudo apt install jq"
        exit 1
    fi

    # Check if our hook already exists (handle both ~ and expanded path)
    HOOK_PATTERN="claude-permissions-hook run --config.*claude-permissions.toml"
    EXISTING=$(jq -r '.hooks.PreToolUse // [] | map(select(.hooks[0].command | test("claude-permissions-hook run"))) | length' "$SETTINGS_FILE" 2>/dev/null || echo "0")

    if [[ "$EXISTING" != "0" ]]; then
        echo -e "${YELLOW}Hook already installed, skipping...${NC}"
    else
        # Add hook to existing settings
        jq --argjson hook "$NEW_HOOK" '.hooks.PreToolUse = ((.hooks.PreToolUse // []) + [$hook])' "$SETTINGS_FILE" > "${SETTINGS_FILE}.tmp"
        mv "${SETTINGS_FILE}.tmp" "$SETTINGS_FILE"
        echo -e "${GREEN}✓ Added hook to ${SETTINGS_FILE}${NC}"
    fi
else
    # Create new settings file
    cat > "$SETTINGS_FILE" <<EOF
{
  "hooks": {
    "PreToolUse": [
      ${NEW_HOOK}
    ]
  }
}
EOF
    echo -e "${GREEN}✓ Created ${SETTINGS_FILE} with hook${NC}"
fi

echo
echo -e "${GREEN}Setup complete!${NC}"
echo
echo "Config location: ${CONFIG_FILE}"
echo "Binary location: ${BINARY_PATH}"
echo
echo "To customize, edit: ${CONFIG_FILE}"
