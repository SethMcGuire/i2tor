{ lib
, buildGoModule
, fetchFromGitHub
, pkg-config
, makeWrapper
, libGL
, libx11
, libxcursor
, libxfixes
, libxinerama
, libxrandr
, libxi
, libxxf86vm
, fontconfig
, freetype
, gnupg
}:

buildGoModule rec {
  pname = "i2tor";
  version = "0.1.8";

  src = fetchFromGitHub {
    owner = "SethMcGuire";
    repo = "i2tor";
    rev = "v${version}";
    hash = "sha256-rbzonene9x0tQfaTA7giMkwUOiRE96LhhL9jESt+HWM=";
  };

  vendorHash = "sha256-CKWoyoZntkOpPi3Wvwmdi5U7CitfWE7x5ZQT72mG1iE=";

  subPackages = [ "cmd/i2tor" ];

  doCheck = false; # integration tests require a live I2P process

  nativeBuildInputs = [
    pkg-config
    makeWrapper
  ];

  buildInputs = [
    libGL
    libx11
    libxcursor
    libxfixes
    libxinerama
    libxrandr
    libxi
    libxxf86vm
    fontconfig
    freetype
  ];

  postInstall = ''
    wrapProgram $out/bin/i2tor \
      --prefix PATH : ${lib.makeBinPath [ gnupg ]}

    install -Dm644 packaging/linux/i2tor.desktop \
      $out/share/applications/i2tor.desktop
    install -Dm644 i2tor.png \
      $out/share/icons/hicolor/512x512/apps/i2tor.png
  '';

  meta = {
    description = "Desktop launcher that combines Tor Browser and Java I2P";
    homepage = "https://github.com/SethMcGuire/i2tor";
    license = lib.licenses.mit;
    maintainers = [ ]; # add your nixpkgs handle here
    platforms = [ "x86_64-linux" ];
    mainProgram = "i2tor";
  };
}
