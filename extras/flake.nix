{
  description = "Ask with every maintained provider extra";

  inputs = {
    ask.url = "path:..";
    nixpkgs.follows = "ask/nixpkgs";
    runtime.url = "github:NousResearch/hermes-agent";
  };

  outputs =
    {
      self,
      ask,
      nixpkgs,
      runtime,
      ...
    }:
    let
      supportedSystems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      eachSystem = nixpkgs.lib.genAttrs supportedSystems;
      standaloneName = builtins.baseNameOf ./hermes;
    in
    {
      formatter = eachSystem (system: nixpkgs.legacyPackages.${system}.nixfmt);

      packages = eachSystem (
        system:
        let
          lib = nixpkgs.lib;
          pkgs = nixpkgs.legacyPackages.${system};
          rootProviders = ask.packages.${system}.extras.providers;
          providers = rootProviders // {
            "${standaloneName}" = import ./hermes/package.nix {
              inherit pkgs;
              runtime = runtime.packages.${system}.minimal;
            };
          };
          extras = pkgs.symlinkJoin {
            name = "ask-all-extras";
            paths = lib.attrValues providers;
            passthru = { inherit providers; };
          };
          full = pkgs.symlinkJoin {
            name = "ask-all-providers";
            paths = [
              ask.packages.${system}.ask
              extras
            ];
          };
          providerOutputs = lib.mapAttrs' (
            name: package: lib.nameValuePair "provider-${name}" package
          ) providers;
        in
        {
          inherit extras full;
          default = full;
        }
        // providerOutputs
      );

      checks = eachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          packages = self.packages.${system};
        in
        {
          default = pkgs.runCommand "ask-all-provider-validation" { nativeBuildInputs = [ pkgs.jq ]; } ''
            export HOME="$TMPDIR/home"
            export XDG_CONFIG_HOME="$TMPDIR/config"
            export XDG_DATA_HOME="$TMPDIR/data"
            export XDG_DATA_DIRS="${packages.full}/share"
            export ASK_PROVIDER_PATH=""
            export PATH="${packages.full}/bin:${pkgs.jq}/bin:${pkgs.coreutils}/bin"
            mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_DATA_HOME"
            test "$(ask provider list --json | jq 'length')" -eq ${toString (builtins.length (builtins.attrNames packages.extras.providers))}
            ask provider validate
            touch "$out"
          '';
        }
      );
    };
}
