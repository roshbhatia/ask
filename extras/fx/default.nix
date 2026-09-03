{ callPackage, mkProvider }:
mkProvider {
  name = "fx";
  runtime = callPackage ./runtime.nix { };
  command = "fx";
  manifest = ./provider.yaml;
}
