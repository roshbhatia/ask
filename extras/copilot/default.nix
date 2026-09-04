{
  github-copilot-cli,
  mkProvider,
}:
mkProvider {
  name = "copilot";
  runtime = github-copilot-cli;
  manifest = ./provider.yaml;
}
