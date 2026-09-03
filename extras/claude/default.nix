{
  claude-code,
  mkProvider,
}:
mkProvider {
  name = "claude";
  runtime = claude-code;
  command = "claude";
  manifest = ./provider.yaml;
  adapterSubpackage = "./extras/claude";
}
