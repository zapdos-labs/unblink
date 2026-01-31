{
  description = "Unblink - AI-powered camera monitoring";

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

            bun
            nodejs

            buf

            tmux

            # For video processing (go2rtc, webrtc)
            ffmpeg
          ];

          shellHook = ''
            echo "Unblink dev environment ready"
            echo "  - Go: $(go version)"
            echo "  - Bun: $(bun --version)"
            echo ""
            echo "Run 'make install' to install dependencies"
          '';
        };
      });
}
