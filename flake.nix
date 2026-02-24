{
  description = "Unblink V2 - AI-powered camera monitoring with Render deployment";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            gotools
            go-tools
            air

            bun
            nodejs

            buf
            protoc-gen-es
            protoc-gen-go
            protoc-gen-connect-go

            tmux

            # For video processing (go2rtc, webrtc)
            ffmpeg

            # Docker for deployment
            docker
            docker-compose
          ];

          shellHook = ''
            echo "Unblink V2 dev environment ready"
            echo "  - Go: $(go version)"
            echo "  - Bun: $(bun --version)"
            echo ""
            echo "Run 'make install' to install dependencies"
            echo "Run 'make typecheck' to typecheck the code"
            echo "Run 'make docker-build' to build Docker image"
          '';
        };
      });
}
