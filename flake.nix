{
  description = "Durable A2UI — Go + Temporal Workflow Streams + Google ADK + A2A Protocol (Hackathon HQ)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            # Go
            go

            # Node (for React Presenter & Audience Kiosk)
            nodejs_22

            # Temporal
            temporal-cli

            # Tunneling (for live audience phone QR demos)
            cloudflared

            # Recording (Playwright + ffmpeg)
            playwright-driver.browsers
            ffmpeg

            # Tools
            git
            jq
            curl
          ];

          shellHook = ''
            if [ -d "$HOME/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.10.darwin-arm64/bin" ]; then
              export PATH="$HOME/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.10.darwin-arm64/bin:$PATH"
            fi
            export GONOSUMDB="*"
            export PLAYWRIGHT_BROWSERS_PATH=${pkgs.playwright-driver.browsers}
            export PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=true

            echo "⚡ durable-a2ui (Go + ADK + A2A + A2UI + Temporal) dev shell"
            echo "   go:          $(go version)"
            echo "   node:        $(node --version)"
            echo "   temporal:    $(temporal --version 2>&1 | head -1)"
            echo "   cloudflared: $(cloudflared --version 2>&1 | head -1)"
            echo ""
            echo "Run ./scripts/setup.sh to install dependencies"
            echo "Run ./scripts/run-demo.sh to start the demo"
          '';
        };
      });
}
