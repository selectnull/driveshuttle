# Build for a uname platform (Darwin or Linux); build both when omitted.
build target="":
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p build

    build_macos() {
        GOOS=darwin GOARCH=arm64 go build -o build/backdrive-macos-arm64 .
        GOOS=darwin GOARCH=amd64 go build -o build/backdrive-macos-amd64 .
    }

    build_linux() {
        GOOS=linux GOARCH=arm64 go build -o build/backdrive-linux-arm64 .
        GOOS=linux GOARCH=amd64 go build -o build/backdrive-linux-amd64 .
    }

    case "{{target}}" in
        Darwin)
            build_macos
            ;;
        Linux)
            build_linux
            ;;
        "")
            build_macos
            build_linux
            ;;
        *)
            echo "Usage: just build [Darwin|Linux]" >&2
            exit 2
            ;;
    esac

# Delete the build directory and all of its contents.
clean:
    rm -rf build
