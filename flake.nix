{
  description = "durable-a2ui – The Agent Interoperability Stack: Go + ADK + A2A + A2UI + Temporal";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        # golangci-lint in nixos-25.11 is built with Go 1.25; override to
        # match our Go 1.26 toolchain so it can lint go 1.26 modules.
        golangci-lint-126 = pkgs.golangci-lint.override {
          buildGo125Module = pkgs.buildGo126Module;
        };
        # Same for govulncheck — needs Go 1.26 to analyse go 1.26 sources.
        govulncheck-126 = pkgs.govulncheck.override {
          buildGoLatestModule = pkgs.buildGo126Module;
        };
      in
      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go_1_26
            golangci-lint-126
            gnumake
            git
            lefthook
            editorconfig-checker

            # Security scanning (local fast-feedback)
            govulncheck-126
            gosec
            gitleaks
            trivy
            syft

            # Demo & frontend runtime
            nodejs_22
            temporal-cli
            cloudflared
            jq
            curl
          ];

          shellHook = ''
            export GONOSUMDB="*"
            echo "⚡ durable-a2ui (Go + ADK + A2A + A2UI + Temporal) dev shell"
            echo "   go:          $(go version)"
            echo "   node:        $(node --version)"
            echo "   temporal:    $(temporal --version 2>/dev/null | head -n1 || echo 'ready')"
            echo "   lefthook:    $(lefthook version 2>/dev/null || echo 'ready')"
            # Auto-install git hooks on shell entry (idempotent, no output if unchanged).
            lefthook install > /dev/null 2>&1 || true
          '';
        };
      });
}
