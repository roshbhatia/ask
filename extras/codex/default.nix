{ codex, mkProvider }:
mkProvider {
  name = "codex";
  runtime = codex;
  command = "codex";
  manifest = ./provider.yaml;
  adapterSubpackage = "./extras/codex";
}
