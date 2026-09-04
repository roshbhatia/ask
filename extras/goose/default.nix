{ goose-cli, mkProvider }:
mkProvider {
  name = "goose";
  runtime = goose-cli;
  manifest = ./provider.yaml;
}
