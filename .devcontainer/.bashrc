#!/bin/bash

# Source global definitions
if [ -f /etc/bashrc ]; then
    . /etc/bashrc
fi

# # Automatically enter nix-shell when starting a terminal
# if [ -f /workspaces/ownwarden/shell.nix ] && [ -z "$IN_NIX_SHELL" ]; then
#     echo "Entering Nix shell environment..."
#     exec nix-shell /workspaces/ownwarden/shell.nix
# fi