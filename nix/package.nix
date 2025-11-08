{
  lib,
  buildGoModule,
  dockerTools,
  ...
}:
dockerTools.buildImage (
  lib.fix (finalAttrs: {
    name = "kube-janitor";
    tag = "latest";

    copyToRoot = buildGoModule {
      pname = "kube-janitor";
      version = "0.1.0";
      src = ../.;
      vendorHash = null;
      goPackagePath = "github.com/r0chd/kube-janitor";
      meta = {
        description = "Simple command-line snippet manager, written in Go";
        homepage = "https://github.com/knqyf263/pet";
        license = lib.licenses.mit;
      };
    };

    config = {
      Cmd = [ ];
      WorkingDir = "/";
      ExposedPorts = {
        "8080/tcp" = { };
      };
    };
  })
)
