{
  buildGo,
  lib,
  pkgs,
  version,
}:
{
  name,
  runtime,
  command,
  manifest,
  adapterSubpackage ? null,
}:
let
  textAdapter = buildGo {
    name = "ask-provider-text";
    subPackage = "./cmd/ask-provider-text";
    builtName = "ask-provider-text";
  };
  adapter =
    if adapterSubpackage == null then
      textAdapter
    else
      buildGo {
        name = "ask-provider-${name}-raw";
        subPackage = adapterSubpackage;
        builtName = name;
      };
  executable = "ask-provider-${name}";
  entry = pkgs.writeShellApplication {
    name = executable;
    runtimeInputs = [
      runtime
      adapter
    ];
    text =
      if adapterSubpackage == null then
        ''
          exec ask-provider-text "$@"
        ''
      else
        ''
          exec ask-provider-${name}-raw "$@"
        '';
  };
in
pkgs.runCommand "ask-provider-${name}-${version}" { } ''
  mkdir -p "$out/bin" "$out/share/ask/providers/${name}"
  ln -s ${lib.getExe entry} "$out/bin/${executable}"
  ln -s ${lib.getExe runtime} "$out/bin/${command}"
  install -m 0444 ${manifest} "$out/share/ask/providers/${name}/provider.yaml"
''
