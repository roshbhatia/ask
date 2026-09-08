{
  devin-cli,
  mkProvider,
}:
mkProvider {
  name = "devin";
  runtime = devin-cli;
  manifest = ./provider.yaml;
}
