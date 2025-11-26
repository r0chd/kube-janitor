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
      vendorHash = "sha256-goBiiw+O6li+l7zYTHgGY0+apUhi8yg2ESK3u96A9BA=";
      goPackagePath = "github.com/r0chd/kube-janitor";
      modFlags = [ "-mod=mod" ];
      meta = {
        description = "";
        homepage = "https://github.com/r0chd/kube-janitor.git";
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
