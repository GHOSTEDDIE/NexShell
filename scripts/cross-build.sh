#!/bin/sh
set -eu
native_arch="$(go env GOARCH)"
go build -mod=readonly -trimpath -ldflags='-s -w' -o "/out/nexshell-linux-$native_arch" ./cmd/nexshell
GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 go build -mod=readonly -trimpath -ldflags='-s -w -H windowsgui' -o /out/NexShell-windows-amd64.exe ./cmd/nexshell
