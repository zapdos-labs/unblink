#!/usr/bin/env bash

# tmux.dev.sh - Development tmux session with server + node

# Source the library
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/tmux.lib.sh"

# Configuration
SESSION_NAME="unb-dev"
PROJECT_DIR="$(tmux_get_project_dir)"
NODE_CONFIG="node.dev.json"

# Main script
if ! tmux has-session -t "=$SESSION_NAME" 2>/dev/null; then
  echo "Creating and attaching to new tmux session '$SESSION_NAME'."

  # Initialize session (no windows)
  tmux_session_init "$SESSION_NAME"

  # Configure session (mouse, keybindings)
  tmux_configure_session "$SESSION_NAME"

  # Create all windows using the same function
  tmux_create_window "$SESSION_NAME" "server" "$PROJECT_DIR" "go run ./cmd/server/main.go"
  tmux_create_window "$SESSION_NAME" "node" "$PROJECT_DIR" "sleep 8 && go run ./cmd/node/main.go -config $NODE_CONFIG"
  tmux_create_window "$SESSION_NAME" "app" "$PROJECT_DIR" "cd app && bun dev"

  # Attach to server window
  tmux_session_attach "$SESSION_NAME" "server"
else
  echo "Attaching to existing tmux session '$SESSION_NAME'."
  tmux attach-session -t "$SESSION_NAME"
fi
