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
  ];

  # Set any environment variables if needed
  # environment.variables = {
  #   EXAMPLE_VAR = "value";
  # };
}