{ pi-coding-agent, mkProvider }:
mkProvider {
  name = "pi";
  runtime = pi-coding-agent;
  manifest = ./provider.yaml;
}
