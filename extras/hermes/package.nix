{
  pkgs,
  runtime,
}:
let
  adapter = pkgs.buildGoModule {
    pname = "ask-provider-hermes";
    version = "0.4.0";
    src = ./.;
    vendorHash = null;
    meta.mainProgram = "ask-provider-hermes";
  };
  entry = pkgs.writeShellApplication {
    name = "ask-provider-hermes";
    runtimeInputs = [ runtime ];
    text = ''
      exec ${pkgs.lib.getExe adapter} "$@"
    '';
  };
in
pkgs.runCommand "ask-provider-hermes-0.4.0" { } ''
  mkdir -p "$out/bin" "$out/share/ask/providers/hermes"
  ln -s ${pkgs.lib.getExe entry} "$out/bin/ask-provider-hermes"
  ln -s ${pkgs.lib.getExe runtime} "$out/bin/hermes"
  install -m 0444 ${./provider.yaml} "$out/share/ask/providers/hermes/provider.yaml"
''
