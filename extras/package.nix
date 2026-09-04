{
  buildGo,
  lib,
  pkgs,
  version,
}:
{
  name,
  runtime,
  manifest,
  adapterSubpackage ? "./extras/${name}",
}:
let
  adapter = buildGo {
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
    text = ''
      exec ask-provider-${name}-raw "$@"
    '';
  };
  adapterPackage = pkgs.runCommand "ask-provider-${name}-adapter-${version}" { } ''
    mkdir -p "$out/bin" "$out/share/ask/providers/${name}"
    ln -s ${lib.getExe entry} "$out/bin/${executable}"
    install -m 0444 ${manifest} "$out/share/ask/providers/${name}/provider.yaml"
  '';
in
pkgs.symlinkJoin {
  name = "ask-provider-${name}-${version}";
  paths = [
    adapterPackage
    runtime
  ];
  passthru = {
    adapter = adapterPackage;
    providerRuntime = runtime;
  };
  meta = adapterPackage.meta or { };
}
