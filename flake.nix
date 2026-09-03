{
  description = "Composable agent question CLI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs =
    {
      self,
      nixpkgs,
      ...
    }:
    let
      supportedSystems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      eachSystem = nixpkgs.lib.genAttrs supportedSystems;
      providerDirectories = nixpkgs.lib.filterAttrs (
        name: type:
        type == "directory"
        && builtins.pathExists (./extras + "/${name}/default.nix")
        && builtins.pathExists (./extras + "/${name}/provider.yaml")
      ) (builtins.readDir ./extras);
      providerNames = builtins.attrNames providerDirectories;
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
          lib = nixpkgs.lib;
          pkgs = import nixpkgs {
            inherit system;
            config.allowUnfree = true;
          };
          version = "0.4.0";
          vendorHash = "sha256-8gN29eM8LPF3UkNxpx86sa7trSdBt0slgZ1Oh3Ygd7g=";
          buildGo =
            {
              name,
              subPackage,
              builtName ? builtins.baseNameOf subPackage,
              check ? false,
            }:
            pkgs.buildGoModule {
              pname = name;
              inherit version vendorHash;
              src = ./.;
              subPackages = [ subPackage ];
              nativeCheckInputs = lib.optionals check [
                pkgs.cue
                pkgs.ripgrep
              ];
              doCheck = check;
              checkPhase = lib.optionalString check ''
                runHook preCheck
                go test -race ./...
                ${pkgs.bash}/bin/bash ./hack/generate.sh --check
                runHook postCheck
              '';
              postInstall = lib.optionalString (builtName != name) ''
                mv "$out/bin/${builtName}" "$out/bin/${name}"
              '';
              meta.mainProgram = name;
            };
          ask =
            (buildGo {
              name = "ask";
              subPackage = ".";
              builtName = "ask";
              check = true;
            }).overrideAttrs
              (old: {
                nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [ pkgs.installShellFiles ];
                postInstall = (old.postInstall or "") + ''
                  installShellCompletion \
                    --cmd ask \
                    --bash <("$out/bin/ask" completion bash) \
                    --fish <("$out/bin/ask" completion fish) \
                    --zsh <("$out/bin/ask" completion zsh)
                  mkdir -p "$out/share/nushell/vendor/autoload"
                  "$out/bin/ask" completion nu > "$out/share/nushell/vendor/autoload/ask.nu"
                '';
              });
          textAdapter = buildGo {
            name = "ask-provider-text";
            subPackage = "./cmd/ask-provider-text";
            builtName = "ask-provider-text";
          };
          mkProvider = import ./extras/package.nix {
            inherit
              buildGo
              lib
              pkgs
              version
              ;
          };
          providerScope = pkgs // {
            inherit mkProvider;
          };
          providers = lib.genAttrs providerNames (
            name: lib.callPackageWith providerScope (./extras + "/${name}/default.nix") { }
          );
          extras = pkgs.symlinkJoin {
            name = "ask-extras-${version}";
            paths = map (provider: provider.adapter) (lib.attrValues providers);
            passthru.providers = providers;
          };
          full = pkgs.symlinkJoin {
            name = "ask-full-${version}";
            paths = [
              ask
              extras
            ];
          };
          providerOutputs = lib.mapAttrs' (
            name: package: lib.nameValuePair "provider-${name}" package
          ) providers;
        in
        {
          inherit ask extras full;
          provider-text = textAdapter;
          default = ask;
        }
        // providerOutputs
      );

      apps = eachSystem (system: {
        default = {
          type = "app";
          program = "${nixpkgs.lib.getExe self.packages.${system}.default}";
        };
      });

      checks = eachSystem (
        system:
        let
          lib = nixpkgs.lib;
          pkgs = nixpkgs.legacyPackages.${system};
          packages = self.packages.${system};
          names = providerNames;
          providerCheck =
            name:
            let
              package = packages."provider-${name}";
            in
            pkgs.runCommand "ask-provider-${name}-validation" { nativeBuildInputs = [ pkgs.jq ]; } ''
              export HOME="$TMPDIR/home"
              export XDG_CONFIG_HOME="$TMPDIR/config"
              export XDG_DATA_HOME="$TMPDIR/data"
              export XDG_DATA_DIRS="${package}/share"
              unset ASK_CONFIG ASK_PROVIDER ASK_PROVIDER_DEFAULT ASK_PROVIDERS_DIRECTORY
              export ASK_PROVIDER_PATH=""
              export PATH="${package}/bin:${packages.ask}/bin:${pkgs.jq}/bin:${pkgs.gnugrep}/bin:${pkgs.coreutils}/bin"
              mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_DATA_HOME"

              test -x "${package}/bin/ask-provider-${name}"
              test -f "${package}/share/ask/providers/${name}/provider.yaml"
              ask provider validate "${name}"
              test "$(ask provider list --json | jq 'length')" -eq 1
              ask provider list | grep -F "command: ask-provider-${name}"
              touch "$out"
            '';
          isolatedChecks = map providerCheck names;
        in
        {
          default = packages.ask;
          core-neutral =
            pkgs.runCommand "ask-provider-neutral-source" { nativeBuildInputs = [ pkgs.ripgrep ]; }
              ''
                cd ${./.}
                ${pkgs.bash}/bin/bash ./hack/audit-provider-neutral.sh
                touch "$out"
              '';
          no-providers =
            pkgs.runCommand "ask-no-providers"
              {
                nativeBuildInputs = [
                  pkgs.findutils
                  pkgs.jq
                ];
              }
              ''
                    export HOME="$TMPDIR/home"
                export XDG_CONFIG_HOME="$TMPDIR/config"
                export XDG_DATA_HOME="$TMPDIR/data"
                export XDG_DATA_DIRS="$TMPDIR/system-data"
                unset ASK_CONFIG ASK_PROVIDER ASK_PROVIDER_DEFAULT ASK_PROVIDERS_DIRECTORY
                export ASK_PROVIDER_PATH=""
                    mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_DATA_HOME" "$XDG_DATA_DIRS"
                    test "$(${packages.ask}/bin/ask provider list --json | jq 'length')" -eq 0
                    test ! -e "${packages.ask}/share/ask/providers"
                    if ${pkgs.findutils}/bin/find "${packages.ask}/bin" -name 'ask-provider*' -print -quit | grep -q .; then
                      exit 1
                    fi
                    if ${packages.ask}/bin/ask -q "test prompt" >answer 2>error; then
                      exit 1
                    fi
                grep -F "say which agent to run" error
                    touch "$out"
              '';
          providers = pkgs.linkFarm "ask-provider-validations" (
            lib.imap0 (index: path: {
              name = builtins.elemAt names index;
              inherit path;
            }) isolatedChecks
          );
          full = pkgs.runCommand "ask-full-provider-layout" { nativeBuildInputs = [ pkgs.jq ]; } ''
            export HOME="$TMPDIR/home"
            export XDG_CONFIG_HOME="$TMPDIR/config"
            export XDG_DATA_HOME="$TMPDIR/data"
            export XDG_DATA_DIRS="${packages.full}/share"
            unset ASK_CONFIG ASK_PROVIDER ASK_PROVIDER_DEFAULT ASK_PROVIDERS_DIRECTORY
            export ASK_PROVIDER_PATH=""
            mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_DATA_HOME"
            test "$(${packages.full}/bin/ask provider list --json | jq 'length')" -eq ${toString (builtins.length names)}
            touch "$out"
          '';
          provider-aggregate-boundary =
            pkgs.runCommand "ask-provider-aggregate-boundary" { nativeBuildInputs = [ pkgs.findutils ]; }
              ''
                test "$(find ${packages.extras}/bin -type l -o -type f | wc -l | tr -d ' ')" -eq ${toString (builtins.length names)}
                if find ${packages.extras}/bin -mindepth 1 -maxdepth 1 -type l -exec basename {} \; \
                  | grep -Ev '^ask-provider-'; then
                  exit 1
                fi
                touch "$out"
              '';
        }
      );

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
