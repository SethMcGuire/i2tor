{
  description = "Desktop launcher combining Tor Browser and Java I2P";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
    in {
      packages.${system} = {
        i2tor = pkgs.callPackage ./packaging/nix/package.nix { };
        default = self.packages.${system}.i2tor;
      };

      devShells.${system}.default = pkgs.mkShell {
        nativeBuildInputs = with pkgs; [ go pkg-config ];
        buildInputs = with pkgs; [
          libGL
          xorg.libX11
          xorg.libXcursor
          xorg.libXfixes
          xorg.libXinerama
          xorg.libXrandr
          xorg.libXi
          fontconfig
          freetype
          gnupg
        ];
      };
    };
}
