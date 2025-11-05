#!/bin/bash

# Set the name for your tmux session
SESSION_NAME="unblink"

# Check if a tmux session with the EXACT name already exists.
# The '=' forces an exact match, preventing it from matching "unblink_engine".
if ! tmux has-session -t "=$SESSION_NAME" 2>/dev/null; then
  # Session does NOT exist.
  echo "Creating and attaching to new tmux session '$SESSION_NAME'."

  # Create a new session (without -d) and run the command.
  # Omitting -d makes tmux create and attach to the session immediately.
  tmux new-session -s $SESSION_NAME 'NODE_ENV=development bun index.ts'
else
  # Session DOES exist.
  echo "Attaching to existing tmux session '$SESSION_NAME'."

  # Attach to the existing session.
  tmux attach-session -t $SESSION_NAME
fi