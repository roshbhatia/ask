{
  antigravity-cli,
  mkProvider,
}:
mkProvider {
  name = "antigravity";
  runtime = antigravity-cli;
  command = "agy";
  manifest = ./provider.yaml;
}
