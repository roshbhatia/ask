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
          adapter = pkgs.buildGoModule {
            pname = "ask-provider-hermes";
            version = "0.4.0";
            src = ./.;
            vendorHash = null;
            meta.mainProgram = "ask-provider-hermes";
          };
          hermes = runtime.packages.${system}.minimal;
          entry = pkgs.writeShellApplication {
            name = "ask-provider-hermes";
            runtimeInputs = [ hermes ];
            text = ''
              exec ${pkgs.lib.getExe adapter} "$@"
            '';
          };
        in
        {
          default = pkgs.runCommand "ask-provider-hermes-0.4.0" { } ''
            mkdir -p "$out/bin" "$out/share/ask/providers/hermes"
            ln -s ${pkgs.lib.getExe entry} "$out/bin/ask-provider-hermes"
            ln -s ${pkgs.lib.getExe hermes} "$out/bin/hermes"
            install -m 0444 ${./provider.yaml} "$out/share/ask/providers/hermes/provider.yaml"
          '';
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
