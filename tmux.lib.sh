#!/usr/bin/env bash

# tmux.lib.sh - Shared functions for tmux session management
# Source this file in your tmux scripts to use these functions

# Get project root directory
tmux_get_project_dir() {
  echo "$(cd "$(dirname "${BASH_SOURCE[1]}")" && pwd)"
}

# Initialize tmux session if it doesn't exist
# Creates a session with an initial "__init__" window that will be replaced by the first real window
# Usage: tmux_session_init "$SESSION_NAME"
tmux_session_init() {
  local session=$1

  if ! tmux has-session -t "=$session" 2>/dev/null; then
    tmux new-session -s "$session" -d -n "__init__"
    return 0
  else
    return 1
  fi
}

# Configure tmux session options
# Usage: tmux_configure_session "$SESSION_NAME"
tmux_configure_session() {
  local session=$1

  # Enable mouse support for clickable tabs and panes
  tmux set-option -g mouse on
  tmux set-option -t "$session" mouse on

  # Bind 'x' to kill current window
  tmux bind-key x kill-window

  # Bind 'K' to kill all tmux sessions with confirmation
  tmux bind-key K confirm-before -p "kill all tmux sessions? (y/n)" "run-shell 'tmux kill-server'"
}

# Create a new window and run a command
# If the session has an "__init__" window, renames it instead of creating a new one
# Usage: tmux_create_window "$SESSION" "$WINDOW_NAME" "$WORKING_DIR" "$COMMAND"
tmux_create_window() {
  local session=$1
  local window_name=$2
  local working_dir=$3
  local command=$4

  # Check if there's an __init__ window to reuse
  if tmux list-windows -t "$session" -F "#{window_name}" 2>/dev/null | grep -q "^__init__$"; then
    # Rename the init window instead of creating a new one
    tmux rename-window -t "$session:__init__" "$window_name"
  else
    # Create a new window
    tmux new-window -t "$session" -n "$window_name"
  fi

  tmux send-keys -t "$session:$window_name" "cd \"$working_dir\" && $command" C-m
}

# Attach to session
# Usage: tmux_session_attach "$SESSION_NAME" "$DEFAULT_WINDOW"
tmux_session_attach() {
  local session=$1
  local default_window=${2:-server}

  tmux select-window -t "$session:$default_window"
  tmux attach-session -t "$session"
}
