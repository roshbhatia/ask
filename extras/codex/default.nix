{ codex, mkProvider }:
mkProvider {
  name = "codex";
  runtime = codex;
  manifest = ./provider.yaml;
  adapterSubpackage = "./extras/codex";
}
