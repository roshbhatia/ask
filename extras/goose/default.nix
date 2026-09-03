{ goose-cli, mkProvider }:
mkProvider {
  name = "goose";
  runtime = goose-cli;
  command = "goose";
  manifest = ./provider.yaml;
}
