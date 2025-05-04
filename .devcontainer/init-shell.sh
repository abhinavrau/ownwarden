#!/bin/bash

echo "Initializing Nix development environment..."
if [ -f /workspaces/ownwarden/shell.nix ]; then
  echo "Loading shell.nix environment..."
  exec nix-shell /workspaces/ownwarden/shell.nix
else
  echo "No shell.nix found!"
fi