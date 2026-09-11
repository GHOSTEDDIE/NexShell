# Repository Guidelines

## Project Structure & Module Organization

Native Go/Fyne SSH client with Eino DeepAgent.

- `cmd/nexshell/`: desktop entry point and embedded icon assets.
- `internal/ui/`, `terminal/`, and `platforminput/`: workbench, terminal rendering, and native input integration.
- `internal/remote/`, `transfer/`, and `localfiles/`: shared SSH/SFTP services, Go ZMODEM, and local file access.
- `internal/agent/`, `domain/`, and `store/`: agent runtime, types, SQLite, and credentials. Built-in Skills: `internal/agent/runbooks/`.
- `tests/integration/` and `tests/fixture/`: Linux integration tests and Docker SSH fixture. Unit tests accompany implementation files.
- `third_party/`: patched terminal and GLFW modules. Packaging: `scripts/`; validation records: `docs/evidence/`; artifacts: `bin/` and `dist/`.

## Build, Test, and Development Commands

Use Go from `go.mod`, a C compiler, and graphics dependencies listed in `.github/workflows/ci.yml`.

- `go run ./cmd/nexshell -data-dir /tmp/nexshell-dev`: launch with separate application data.
- `go build ./cmd/nexshell`: compile the native client.
- `go test -race -tags ci ./...`: run unit and Fyne test-driver regressions with race detection.
- `go vet -tags ci ./...`: perform static checks.
- `docker build -t nexshell-test-sshd:local tests/fixture`: prepare the SSH fixture.
- `go test -race -tags integration ./tests/integration -v -count=1`: exercise actual SSH, transfer, and agent workflows.
- `bash scripts/package.sh`: package for the host platform.

## Coding Style & Naming Conventions

Run `gofmt` on changed Go files; use standard Go tabs, lowercase package names, exported `CamelCase` identifiers, and `*_test.go` files. Reuse shared services for UI and agent operations. Keep network/model work off the UI thread; dispatch UI changes through `fyne.Do`. Write Chinese UI copy about operations, with technical explanations in documentation.

## Testing Guidelines

Use Go’s `testing` package and Fyne’s test driver. Use `TestBehavior` and `BenchmarkBehavior` names. Reproduce defects and rerun correct samples after logic changes. Terminal dependency changes also require `(cd third_party/vt && go test ./...)`. Test behavior and failures; no numeric coverage threshold is configured. Record commands and untested platforms; distinguish deterministic tests from explicitly authorized real-model tests.

## Commit & Pull Request Guidelines

Use Chinese, behavior-focused subjects matching recent commits: `新增本地文件读取与部署包上传能力`. Use at least 100 Chinese characters per commit message, explaining additions and fixes versus the preceding version. PRs should describe the problem, resulting behavior, validation, relevant issues, and screenshots for UI changes.

## Security & Configuration

Use isolated fixtures for testing. Keep credentials in the system keyring and out of logs. Route agent operations through existing grants, approvals, execution records, and verification; never automatically replay unknown remote outcomes. Preserve third-party licenses when updating dependencies.
