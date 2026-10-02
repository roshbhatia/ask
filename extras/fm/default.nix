{ mkProvider }:
(mkProvider {
  name = "fm";
  manifest = ./provider.yaml;
}).overrideAttrs
  (_: {
    meta.platforms = [ "aarch64-darwin" ];
  })
