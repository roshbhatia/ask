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
            version = "0.4.0";
            src = ./.;
            vendorHash = "sha256-8gN29eM8LPF3UkNxpx86sa7trSdBt0slgZ1Oh3Ygd7g=";
            subPackages = [
              "."
              "./cmd/ask-provider"
              "./cmd/ask-provider-text"
            ];
            nativeBuildInputs = [
              pkgs.cue
              pkgs.installShellFiles
              pkgs.makeWrapper
            ];
            preCheck = ''
              ./hack/generate.sh --check
              go test -race ./...
            '';
            postInstall = ''
              installShellCompletion \
                --cmd ask \
                --bash <("$out/bin/ask" completion bash) \
                --fish <("$out/bin/ask" completion fish) \
                --zsh <("$out/bin/ask" completion zsh)
              mkdir -p "$out/share/nushell/vendor/autoload"
              "$out/bin/ask" completion nu > "$out/share/nushell/vendor/autoload/ask.nu"

              mkdir -p "$out/share/ask/providers"
              while IFS= read -r -d $'\0' manifest; do
                name="$(basename "$(dirname "$manifest")")"
                mkdir -p "$out/share/ask/providers/$name"
                cp "$manifest" "$out/share/ask/providers/$name/provider.yaml"
              done < <(find ${./extras} -mindepth 2 -maxdepth 2 -name provider.yaml -print0)

              wrapProgram "$out/bin/ask" \
                --prefix PATH : "$out/bin" \
                --suffix ASK_PROVIDER_PATH : "$out/share/ask/providers"
            '';
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
              pkgs.cue
              pkgs.ripgrep
              pkgs.charm-freeze
              pkgs.vhs
              pkgs.fish
              pkgs.nushell
              pkgs.shfmt
            ];
            shellHook = ''
              export GOTOOLCHAIN=local
            '';
          };
        }
      );
    };
}
