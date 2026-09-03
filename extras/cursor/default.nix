{ cursor-cli, mkProvider }:
mkProvider {
  name = "cursor";
  runtime = cursor-cli;
  command = "cursor-agent";
  manifest = ./provider.yaml;
}
