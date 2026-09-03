{
  fetchurl,
  lib,
  stdenv,
}:
let
  releases = {
    aarch64-darwin = {
      file = "fx-macos-aarch64.tar.gz";
      hash = "sha256-SXy6vFDFfs+B+K/+BCoH2TDAQYho52eyivraS93QV0g=";
    };
    x86_64-darwin = {
      file = "fx-macos-x86_64.tar.gz";
      hash = "sha256-xFfk70H7z8tncYugeiH14AQYKVEn+ZmA6ozjjZVd1UY=";
    };
    aarch64-linux = {
      file = "fx-linux-aarch64.tar.gz";
      hash = "sha256-Sj+xsBFLik+TPeZPhfsiiAlcF2MaDDyol6oFYB0EmXQ=";
    };
    x86_64-linux = {
      file = "fx-linux-x86_64.tar.gz";
      hash = "sha256-xXh+oEHTtVIexnXxraePMM8bEQIf/KxItJac9b62XEU=";
    };
  };
  release = releases.${stdenv.hostPlatform.system};
  version = "0.0.7";
in
stdenv.mkDerivation {
  pname = "fx-agent";
  inherit version;

  src = fetchurl {
    url = "https://github.com/vercel-labs/fx/releases/download/v${version}/${release.file}";
    inherit (release) hash;
  };
  sourceRoot = ".";

  installPhase = ''
    runHook preInstall
    install -Dm755 fx "$out/bin/fx"
    install -Dm644 LICENSE "$out/share/licenses/fx/LICENSE"
    install -Dm644 THIRD_PARTY_NOTICES.md "$out/share/licenses/fx/THIRD_PARTY_NOTICES.md"
    runHook postInstall
  '';

  meta = {
    description = "Unix-like coding agent";
    homepage = "https://github.com/vercel-labs/fx";
    license = lib.licenses.asl20;
    mainProgram = "fx";
    platforms = builtins.attrNames releases;
  };
}
