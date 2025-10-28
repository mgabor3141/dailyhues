{
  pkgs,
  lib,
  config,
  inputs,
  ...
}:

{
  # https://devenv.sh/basics/
  env.GREET = "dailyhues";

  # https://devenv.sh/packages/
  packages = [ pkgs.git ];

  languages.nix.enable = true;
  devcontainer.enable = true;
  dotenv.enable = true;

  # https://devenv.sh/languages/
  languages.go.enable = true;

  # https://devenv.sh/scripts/
  scripts.hello.exec = ''
    echo ""
    echo "Welcome to $GREET - Bing Wallpaper Color Analysis API"
  '';

  scripts.dev.exec = ''
    echo "Starting development server..."
    go run ./cmd/dailyhues
  '';

  scripts.build.exec = ''
    echo "Building binary..."
    go build -o bin/dailyhues ./cmd/dailyhues
  '';

  scripts.test.exec = ''
    echo "Running tests..."
    go test ${config.git.root}/... -v
  '';

  scripts.lint.exec = ''
    echo "Running linters and formatters..."
    go fmt ${config.git.root}/...
    go vet ${config.git.root}/...
  '';

  scripts.tidy.exec = ''
    echo "Tidying dependencies..."
    go mod tidy
  '';

  scripts.clear_cache.exec = ''
    echo "Clearing request cache..."
    rm -rf ${config.git.root}/cache_data
    rm -rf ${config.git.root}/debug_responses
  '';

  # https://devenv.sh/basics/
  enterShell = ''
    hello
    echo ""
    echo "Available commands:"
    echo "  dev         - Run the development server"
    echo "  build       - Build production binary"
    echo "  test        - Run tests"
    echo "  lint        - Run formatters and linters"
    echo "  tidy        - Clean up go.mod dependencies"
    echo "  clear_cache - Clear server request cache"
    echo ""
  '';

  # https://devenv.sh/tests/
  enterTest = ''
    echo "Running tests"
    go version | grep --color=auto "go"
  '';

  # See full reference at https://devenv.sh/reference/options/
}
