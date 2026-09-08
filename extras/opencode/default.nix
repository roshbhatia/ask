{ opencode, mkProvider }:
mkProvider {
  name = "opencode";
  runtime = opencode;
  manifest = ./provider.yaml;
}
