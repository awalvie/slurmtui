{
  description = "slurmtui: a k9s-style terminal UI for Slurm over slurmrestd";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs, ... }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (s: f nixpkgs.legacyPackages.${s});
    in
    {
      packages = forAllSystems (pkgs: {
        default = pkgs.buildGoModule {
          pname = "slurmtui";
          version = "0.1.0";
          src = self;
          subPackages = [ "cmd/slurmtui" ];
          vendorHash = "sha256-4RYUZGDZ2DzJYDbxTNSapZ3SWTq8/3lHhXRVoCDhCqg=";
          meta = {
            description = "k9s-style terminal UI for Slurm over slurmrestd";
            license = pkgs.lib.licenses.mit;
            mainProgram = "slurmtui";
          };
        };
        slurm-image = pkgs.dockerTools.buildLayeredImage {
          name = "slurmtui-slurm";
          tag = "latest";
          contents = [
            pkgs.slurm
            pkgs.bashInteractive
            pkgs.coreutils
            pkgs.util-linux
            pkgs.dockerTools.fakeNss
            pkgs.dockerTools.caCertificates
          ];
          extraCommands = ''
            mkdir -p tmp run var/spool etc/slurm
            chmod 1777 tmp
          '';
          config = {
            Entrypoint = [
              "bash"
              "/conf/entrypoint.sh"
            ];
            Env = [ "PATH=/bin:/sbin" ];
          };
        };
      });

      apps = forAllSystems (pkgs: {
        openapi-spec = {
          type = "app";
          program = pkgs.lib.getExe (
            pkgs.writeShellApplication {
              name = "openapi-spec";
              runtimeInputs = [ pkgs.git ];
              text = ''
                version=$(${pkgs.slurm}/bin/slurmrestd -V)
                case "$version" in
                "slurm 26.05."*) ;;
                *)
                  echo "openapi-spec: nixpkgs has '$version', but testdata/slurm-26.05 is for Slurm 26.05" >&2
                  exit 1
                  ;;
                esac
                out="$(git rev-parse --show-toplevel)/testdata/slurm-26.05/openapi-slurmctld-v0.0.45.json"
                env -i SLURM_CONF=/dev/null ${pkgs.slurm}/bin/slurmrestd -s slurmctld -d v0.0.45 --generate-openapi-spec > "$out.tmp"
                mv "$out.tmp" "$out"
                echo "openapi-spec: wrote $out from $version"
              '';
            }
          );
        };
      });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.gopls
            pkgs.golangci-lint
            pkgs.just
          ];
        };
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt);
    };
}
