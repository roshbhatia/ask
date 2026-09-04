{
  antigravity-cli,
  mkProvider,
}:
mkProvider {
  name = "antigravity";
  runtime = antigravity-cli;
  manifest = ./provider.yaml;
}
