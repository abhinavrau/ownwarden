#!/bin/bash

# Copy .bashrc to root's home directory
cp /workspaces/ownwarden/.devcontainer/.bashrc /root/.bashrc

# Update channel to ensure nix packages can be found
nix-channel --update

echo "Setup complete! Your Nix development environment is ready."