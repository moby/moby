let _pkgs = import <nixpkgs> { };
in { pkgs ? import (_pkgs.fetchFromGitHub {
  owner = "NixOS";
  repo = "nixpkgs";
  #branch@date: 21.11@2022-02-13
  rev = "560ad8a2f89586ab1a14290f128ad6a393046065";
  sha256 = "0s0dv1clfpjyzy4p6ywxvzmwx9ddbr2yl77jf1wqdbr0x1206hb8";
}) { } }:

with pkgs;

mkShell {
  buildInputs = [
    git
    gnumake
    gnused
    go
    nixfmt
    nodePackages.prettier
    python3Packages.pip
    python3Packages.setuptools
    rufo
    shfmt
    vagrant
  ];
}
