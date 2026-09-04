{ crush, mkProvider }:
let
  crushWithSandboxedTests = crush.overrideAttrs (old: {
    postPatch = (old.postPatch or "") + ''
      substituteInPlace internal/agent/common_test.go \
        --replace-fail \
          'filepath.Join("/tmp/crush-test/", t.Name())' \
          'filepath.Join(os.TempDir(), "crush-test", t.Name())'
    '';
  });
in
mkProvider {
  name = "crush";
  runtime = crushWithSandboxedTests;
  manifest = ./provider.yaml;
}
