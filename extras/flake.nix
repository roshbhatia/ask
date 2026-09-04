{
  description = "Ask with every maintained provider extra";

  inputs = {
    ask.url = "path:..";
    nixpkgs.follows = "ask/nixpkgs";
    standalone-provider.url = "path:./hermes";
    standalone-provider.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs =
    {
      self,
      ask,
      nixpkgs,
      standalone-provider,
      ...
    }:
    let
      supportedSystems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      eachSystem = nixpkgs.lib.genAttrs supportedSystems;
      rootProviderDirectories = nixpkgs.lib.filterAttrs (
        name: type:
        type == "directory"
        && builtins.pathExists (./. + "/${name}/default.nix")
        && builtins.pathExists (./. + "/${name}/provider.yaml")
      ) (builtins.readDir ./.);
      rootProviderNames = builtins.attrNames rootProviderDirectories;
      standaloneName = builtins.baseNameOf ./hermes;
      providerNames = rootProviderNames ++ [ standaloneName ];
    in
    {
      formatter = eachSystem (system: nixpkgs.legacyPackages.${system}.nixfmt);

      packages = eachSystem (
        system:
        let
          lib = nixpkgs.lib;
          pkgs = nixpkgs.legacyPackages.${system};
          rootProviders = lib.genAttrs rootProviderNames (name: ask.packages.${system}."provider-${name}");
          providers = rootProviders // {
            "${standaloneName}" = standalone-provider.packages.${system}.default;
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
            test "$(ask provider list --json | jq 'length')" -eq ${toString (builtins.length providerNames)}
            ask provider validate
            touch "$out"
          '';
        }
      );
    };
}
