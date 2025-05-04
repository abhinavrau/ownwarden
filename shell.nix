{ pkgs ? import <nixpkgs> {} }:

let
    pkgs = import (builtins.fetchTarball {
        url = "https://github.com/NixOS/nixpkgs/archive/42c5e250a8a9162c3e962c78a4c393c5ac369093.tar.gz";
    }) {};

    myPkg = pkgs.opentofu;
in
pkgs.mkShell {
  buildInputs = [
    pkgs.opentofu
    pkgs.pkgs.go
    pkgs.google-cloud-sdk
    # Added shell customization packages
    pkgs.zsh
    pkgs.oh-my-zsh
    pkgs.zsh-powerlevel10k
    pkgs.git
    pkgs.fontconfig
    pkgs.nerdfonts
  ];

  # Set any environment variables if needed
  # environment.variables = {
  #   EXAMPLE_VAR = "value";
  # };
  
  # Set zsh as the default shell for this environment
  shellHook = ''
    export SHELL=${pkgs.zsh}/bin/zsh
    exec $SHELL
  '';
}