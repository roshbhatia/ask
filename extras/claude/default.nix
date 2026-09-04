{
  claude-code,
  mkProvider,
}:
mkProvider {
  name = "claude";
  runtime = claude-code;
  manifest = ./provider.yaml;
  adapterSubpackage = "./extras/claude";
}
