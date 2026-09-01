{
  description = "Composable agent question CLI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    systems.url = "github:nix-systems/default";
  };

  outputs =
    {
      self,
      nixpkgs,
      systems,
      ...
    }:
    let
      eachSystem = nixpkgs.lib.genAttrs (import systems);
    in
    {
      formatter = eachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        pkgs.writeShellApplication {
          name = "ask-format";
          runtimeInputs = [
            pkgs.fd
            pkgs.nixfmt
          ];
          text = ''
            if [ "$#" -gt 0 ] && [ "''${1#-}" = "$1" ]; then
              exec nixfmt "$@"
            fi
            exec fd --extension nix --type file --exec-batch nixfmt "$@"
          '';
        }
      );

      packages = eachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          ask = pkgs.buildGoModule {
            pname = "ask";
            version = "0.1.0";
            src = ./.;
            vendorHash = "sha256-2JZU7jIbgqmUZUOoblhytcqpViQYNoUMKXPM5UawOuw=";
            meta = {
              description = "Send typed questions and input to local agent harnesses";
              homepage = "https://github.com/roshbhatia/ask";
              license = pkgs.lib.licenses.mit;
              mainProgram = "ask";
              platforms = pkgs.lib.platforms.unix;
            };
          };
        in
        {
          inherit ask;
          default = ask;
        }
      );

      apps = eachSystem (system: {
        default = {
          type = "app";
          program = "${nixpkgs.lib.getExe self.packages.${system}.default}";
        };
      });

      checks = eachSystem (system: {
        default = self.packages.${system}.default;
      });

      devShells = eachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShellNoCC {
            packages = [
              pkgs.go
              pkgs.gopls
              pkgs.gotools
              pkgs.go-tools
              pkgs.goreleaser
              pkgs.ripgrep
            ];
            shellHook = ''
              export GOTOOLCHAIN=local
            '';
          };
        }
      );
    };
}
