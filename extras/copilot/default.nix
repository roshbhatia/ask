{
  github-copilot-cli,
  mkProvider,
}:
mkProvider {
  name = "copilot";
  runtime = github-copilot-cli;
  command = "copilot";
  manifest = ./provider.yaml;
}
