#!/bin/bash

# Ensure the 'notes' session exists
if ! tmux has-session -t notes 2>/dev/null; then
  # Create a new session named 'notes', detached
  cd /workspaces/tasks
  tmux new-session -d -s notes
fi
