{ pkgs, lib, config, inputs, ... }:

{
  # https://devenv.sh/basics/
  env.GREET = "wallpaper-highlight";

  # https://devenv.sh/packages/
  packages = [ pkgs.git ];

  # https://devenv.sh/languages/
  languages.go.enable = true;

  # https://devenv.sh/scripts/
  scripts.hello.exec = ''
    echo "🎨 Welcome to $GREET - Bing Wallpaper Color Analysis API"
    echo "Go version: $(go version)"
  '';

  scripts.dev.exec = ''
    echo "Starting development server..."
    go run main.go
  '';

  scripts.build.exec = ''
    echo "Building binary..."
    go build -o bin/wallpaper-highlight
    echo "✓ Binary created at bin/wallpaper-highlight"
  '';

  scripts.test.exec = ''
    echo "Running tests..."
    go test ./... -v
  '';

  scripts.lint.exec = ''
    echo "Running linters and formatters..."
    go fmt ./...
    go vet ./...
  '';

  scripts.tidy.exec = ''
    echo "Tidying dependencies..."
    go mod tidy
  '';

  # https://devenv.sh/basics/
  enterShell = ''
    hello
    echo ""
    echo "Available commands:"
    echo "  dev   - Run the development server"
    echo "  build - Build production binary"
    echo "  test  - Run tests"
    echo "  lint  - Run formatters and linters"
    echo "  tidy  - Clean up go.mod dependencies"
    echo ""
  '';

  # https://devenv.sh/tests/
  enterTest = ''
    echo "Running tests"
    go version | grep --color=auto "go"
  '';

  # See full reference at https://devenv.sh/reference/options/
}
