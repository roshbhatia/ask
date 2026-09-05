{
  description = "Hermes provider for Ask";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    runtime.url = "github:NousResearch/hermes-agent";
  };

  outputs =
    {
      self,
      nixpkgs,
      runtime,
      ...
    }:
    let
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      eachSystem = nixpkgs.lib.genAttrs systems;
    in
    {
      formatter = eachSystem (system: nixpkgs.legacyPackages.${system}.nixfmt);
      packages = eachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = import ./package.nix {
            inherit pkgs;
            runtime = runtime.packages.${system}.minimal;
          };
        }
      );
      checks = eachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          provider = self.packages.${system}.default;
        in
        {
          default = pkgs.runCommand "ask-provider-hermes-contract" { nativeBuildInputs = [ pkgs.jq ]; } ''
            ${provider}/bin/ask-provider-hermes < ${./probe.json} > response.json
            jq -e '.version == "provider/v1" and .status == "ok"' response.json
            touch "$out"
          '';
        }
      );
    };
}
