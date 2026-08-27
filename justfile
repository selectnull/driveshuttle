# Build for darwin or linux; build both when omitted.
build target="":
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p build

    build_macos() {
        GOOS=darwin GOARCH=arm64 go build -o build/driveshuttle-macos-arm64 .
        GOOS=darwin GOARCH=amd64 go build -o build/driveshuttle-macos-amd64 .
    }

    build_linux() {
        GOOS=linux GOARCH=arm64 go build -o build/driveshuttle-linux-arm64 .
        GOOS=linux GOARCH=amd64 go build -o build/driveshuttle-linux-amd64 .
    }

    case "{{target}}" in
        darwin)
            build_macos
            ;;
        linux)
            build_linux
            ;;
        "")
            build_macos
            build_linux
            ;;
        *)
            echo "Usage: just build [darwin|linux]" >&2
            exit 2
            ;;
    esac

# Delete the build directory and all of its contents.
clean:
    rm -rf build
