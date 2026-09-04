{ cursor-cli, mkProvider }:
mkProvider {
  name = "cursor";
  runtime = cursor-cli;
  manifest = ./provider.yaml;
}
