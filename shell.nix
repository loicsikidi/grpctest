let
  # Pin nixpkgs to a specific commit for reproducibility (Go 1.25.1)
  nixpkgs = fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/bce5fe2bb998488d8e7e7856315f90496723793c.tar.gz";
    sha256 = "sha256:0hkpgac4djwffciz171h37zb8xx26q6af1aa0c87233kgvscn1dz";
  };
  pkgs = import nixpkgs {};

  pre-commit-check = pkgs.callPackage ./.nix/precommit.nix {inherit pkgs;};

  genproto = pkgs.writeShellApplication {
    name = "genproto";
    runtimeInputs = [
      pkgs.buf
      pkgs.go
    ];
    text = ''
      go get -u google.golang.org/grpc/cmd/protoc-gen-go-grpc
      if ! buf generate .; then
        echo "proto generate failed ⛔"
        exit 1
      fi
      go mod tidy
      echo "proto generate succeeded 💫"
    '';
  };

  gotest = pkgs.writeShellApplication {
    name = "gotest";
    runtimeInputs = [pkgs.go];
    text = ''
      paths=$(go list ./... | grep -vE '/proto') # exclude generated code
      if ! go test -count=1 -failfast -covermode=count -race -coverprofile=coverage.out -v "$paths"; then
        echo "tests failed ⛔"
        exit 1
      fi
      rm -f coverage.out
      echo "all tests passed 💫"
    '';
  };
in
  pkgs.mkShell {
    buildInputs = pre-commit-check.enabledPackages;
    shellHook = ''
      ${pre-commit-check.shellHook}
    '';

    packages = with pkgs; [
      go
      buf
      delve

      # helper scripts
      genproto
      gotest
    ];
  }
