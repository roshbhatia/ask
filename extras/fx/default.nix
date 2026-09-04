{ callPackage, mkProvider }:
mkProvider {
  name = "fx";
  runtime = callPackage ./runtime.nix { };
  manifest = ./provider.yaml;
}
