{pkgs, ...}: let
  git-hooks-src = import (
    builtins.fetchGit {
      url = "https://github.com/cachix/git-hooks.nix";
      ref = "refs/heads/master";
      rev = "ca5b894d3e3e151ffc1db040b6ce4dcc75d31c37"; # 2025-17-10
    }
  );
  pre-commit-check = git-hooks-src.run {
    src = ./.;
    # If your hooks are intrusive, avoid running on each commit with a default_states like this:
    # default_stages = [ "manual" "pre-push" ];
    hooks = {
      # common
      end-of-file-fixer.enable = true;
      # nix
      alejandra.enable = true;
      # golang
      gofmt.enable = true;
      govet.enable = true;
      golangci-lint = {
        enable = true;
        package = pkgs.golangci-lint;
        extraPackages = [pkgs.go];
        stages = ["pre-push"]; # because it takes a while
      };
      gotest = {
        enable = true;
        package = pkgs.go;
        settings.flags = "-race -failfast -v";
        stages = ["pre-push"]; # because it takes a while
      };
    };
  };
in
  pre-commit-check
