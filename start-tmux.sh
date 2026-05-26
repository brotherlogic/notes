#!/bin/bash

# Ensure the 'prod' session exists
if ! tmux has-session -t tasks 2>/dev/null; then
  # Create a new session named 'prod', detached
  cd /workspaces/tasks
  tmux new-session -d -s tasks
fi
