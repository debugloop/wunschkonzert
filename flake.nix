{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    pre-commit-hooks.url = "github:cachix/git-hooks.nix";
  };
  outputs = inputs @ {
    self,
    nixpkgs,
    ...
  }: let
    supportedSystems = [
      "x86_64-linux"
      "aarch64-linux"
      "x86_64-darwin"
      "aarch64-darwin"
    ];
    eachPkgs = function:
      nixpkgs.lib.genAttrs supportedSystems
      (system: function nixpkgs.legacyPackages.${system});
    eachPkgsSystem = function:
      nixpkgs.lib.genAttrs supportedSystems
      (system: function nixpkgs.legacyPackages.${system} system);
  in {
    formatter = eachPkgs (pkgs: {
      default = pkgs.alejandra;
    });

    packages = eachPkgs (pkgs: {
      default = pkgs.buildGoModule {
        pname = "wunschkonzert";
        version = "0.1.1";
        src = ./.;
        vendorHash = null;
        env.CGO_ENABLED = 0;
      };
    });

    checks = eachPkgsSystem (pkgs: system: {
      pre-commit-check = inputs.pre-commit-hooks.lib.${system}.run {
        src = ./.;
        hooks = {
          alejandra = {
            enable = true;
            name = "nix lint";
            types = ["nix"];
          };
          golangci-lint = {
            enable = true;
            package = pkgs.golangci-lint;
            types = ["go"];
            excludes = [
              "_templ\\.go"
              "^vendor/"
            ];
          };
          templ-format = {
            enable = true;
            name = "templ format";
            entry = "templ fmt -fail";
            files = "\\.templ";
          };
          templ-generate = {
            enable = true;
            name = "templ generate";
            entry = "templ generate ./...";
            files = "\\.templ";
            pass_filenames = false;
          };
        };
      };
    });

    devShells = eachPkgsSystem (pkgs: system: {
      default = pkgs.mkShell {
        buildInputs = self.checks.${system}.pre-commit-check.enabledPackages;
        shellHook = ''
          ${self.checks.${system}.pre-commit-check.shellHook}
          export GOPATH="$(realpath .)/.cache/go";
        '';
        packages = with pkgs; [
          go
          delve
          go-tools
          golangci-lint
          gomodifytags
          gopls
          gotest
          gotests
          gotestsum
          gotools
          templ
          (writeScriptBin "dev" ''
            ARGS="$@"
            ${fd}/bin/fd --min-depth=2 -tf -E'*_templ.go' | ${entr}/bin/entr -cr ${fish}/bin/fish -c "${templ}/bin/templ generate && ${go}/bin/go run ./cmd/server $ARGS"
          '')
          (writeScriptBin "debug" ''
            ARGS="$@"
            ${delve}/bin/dlv --headless --listen 'localhost:2345' debug ./cmd/server -- $ARGS"
          '')
        ];
        env = {
          GOROOT = "${pkgs.go}/share/go";
          GOTOOLCHAIN = "local";
        };
        hardeningDisable = ["fortify"];
        enterShell = ''
          export PATH=$GOPATH/bin:$PATH
        '';
      };
    });

    nixosModules.default = {
      config,
      lib,
      pkgs,
      ...
    }: let
      cfg = config.services.wunschkonzert;
    in {
      options.services.wunschkonzert = {
        enable = lib.mkEnableOption "Enable wunschkonzert service";
        environmentFile = lib.mkOption {
          type = lib.types.nullOr lib.types.path;
          description = ''
            File path containing environment variables in the format of an EnvironmentFile. See {manpage}`systemd.exec(5)`.
          '';
          default = null;
        };
        server = {
          name = lib.mkOption {
            type = lib.types.str;
            description = "Publicly reachable server address";
          };
          listen = lib.mkOption {
            type = lib.types.str;
            default = ":8080";
            description = "Where the main app is listening";
          };
        };
        auth = {
          listen = lib.mkOption {
            type = lib.types.str;
            default = ":8081";
            description = "Where the admin interface is listening";
          };
          client = {
            id = lib.mkOption {
              type = lib.types.str;
              description = "OAuth Client ID for Spotify";
            };
            secret = lib.mkOption {
              type = lib.types.str;
              description = "OAuth Client Secret for Spotify";
            };
          };
        };
        playlist = lib.mkOption {
          type = lib.types.str;
          description = "Playlist ID to add songs to";
        };
        metrics.listen = lib.mkOption {
          type = lib.types.str;
          default = ":9999";
          description = "Where the app exposes its metrics";
        };
        verbose = lib.mkOption {
          type = lib.types.bool;
          default = false;
          description = "Enable verbose logging";
        };
      };

      config = lib.mkIf cfg.enable {
        systemd.services.wunschkonzert = {
          description = "Wunschkonzert Service";
          wantedBy = ["multi-user.target"];
          after = ["network.target"];
          serviceConfig = {
            Type = "simple";
            StateDirectory = "wunschkonzert";
            EnvironmentFile = config.services.wunschkonzert.environmentFile;
            ExecStart = lib.concatStringsSep " \\\n " (
              [
                "${self.packages.${pkgs.system}.default}/bin/server"
                "-server.name=${cfg.server.name}"
                "-server.listen=${cfg.server.listen}"
                "-auth.client.id=${cfg.auth.client.id}"
                "-auth.client.secret=${cfg.auth.client.secret}"
                "-auth.listen=${cfg.auth.listen}"
                "-playlist.id=${cfg.playlist}"
                "-metrics.listen=${cfg.metrics.listen}"
              ]
              ++ (lib.optional cfg.verbose "-verbose")
            );
            Restart = "always";
            DynamicUser = "yes";
          };
        };
      };
    };
  };
}
