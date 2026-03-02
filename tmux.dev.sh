#!/usr/bin/env bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/tmux.lib.sh"

SESSION_NAME="unb-dev"
PROJECT_DIR="$(tmux_get_project_dir)"
ENV_FILE="$PROJECT_DIR/.env"

# Use air for hot reload if available, otherwise fall back to go run
if command -v air >/dev/null 2>&1; then
  BACKEND_CMD="air"
else
  BACKEND_CMD="go run ./cmd/server"
fi

if ! tmux has-session -t "=$SESSION_NAME" 2>/dev/null; then
  echo "Creating tmux session '$SESSION_NAME'"

  tmux_session_init "$SESSION_NAME"
  tmux_configure_session "$SESSION_NAME"

  tmux_create_window "$SESSION_NAME" "app"     "$PROJECT_DIR" "cd app && bun run dev" "$ENV_FILE"
  tmux_create_window "$SESSION_NAME" "backend" "$PROJECT_DIR" "$BACKEND_CMD"          "$ENV_FILE"
  tmux_create_window "$SESSION_NAME" "node"    "$PROJECT_DIR" "sleep 8 && go run ./cmd/unblink-node/main.go -config node.dev.json" "$ENV_FILE"

  tmux_session_attach "$SESSION_NAME" "app"
else
  echo "Attaching to existing session '$SESSION_NAME'"
  tmux attach-session -t "$SESSION_NAME"
fi
