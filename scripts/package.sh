#!/usr/bin/env bash
set -euo pipefail
project_dir="$(cd "$(dirname "$0")/.." && pwd)"
cd "$project_dir"
target_os="$(go env GOOS)"
target_arch="$(go env GOARCH)"
mkdir -p bin dist
binary_name=nexshell
if [[ "$target_os" == windows ]]; then binary_name=nexshell.exe; fi
go build -trimpath -ldflags='-s -w' -o "bin/$binary_name" ./cmd/nexshell
case "$target_os" in
  darwin)
    bundle=dist/NexShell.app
    mkdir -p "$bundle/Contents/MacOS" "$bundle/Contents/Resources"
    cp "bin/$binary_name" "$bundle/Contents/MacOS/nexshell"
    cp scripts/Info.plist "$bundle/Contents/Info.plist"
    cp -f LICENSE NOTICE docs/THIRD_PARTY_LICENSES.txt "$bundle/Contents/Resources/"
    codesign --force --deep --sign - "$bundle"
    ditto -c -k --sequesterRsrc --keepParent "$bundle" "dist/NexShell-darwin-$target_arch.zip"
    ;;
  windows)
    cp "bin/$binary_name" "dist/NexShell-windows-$target_arch.exe"
    cp -f LICENSE NOTICE docs/THIRD_PARTY_LICENSES.txt dist/
    ;;
  linux)
    archive_dir="dist/NexShell-linux-$target_arch"
    mkdir -p "$archive_dir"
    cp "bin/$binary_name" LICENSE NOTICE docs/THIRD_PARTY_LICENSES.txt "$archive_dir/"
    tar -czf "$archive_dir.tar.gz" -C dist "NexShell-linux-$target_arch"
    ;;
  *) printf 'Unsupported packaging target: %s\n' "$target_os" >&2; exit 1 ;;
esac
