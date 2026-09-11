# macOS 窗口与应用退出

macOS 点击红色关闭按钮会隐藏主窗口，保留 SSH 连接、文件传输、助手任务和对话。通过 Dock 或系统重新打开应用时，恢复原窗口并聚焦。Command-Q、应用菜单“退出 NexShell”和系统终止请求进入统一退出流程，停止任务、关闭连接并保存状态。

实现位于 `internal/ui/lifecycle.go`：窗口关闭与应用退出分开处理，复用原有清理顺序，重复退出只执行一次。Windows 和 Linux 关闭主窗口仍按原行为退出应用。

Fyne 自动生成的默认退出动作会转成窗口关闭请求，因此应用显式提供 `IsQuit` 菜单项。macOS 原生适配位于 `internal/platforminput/application_darwin.*`，在 Fyne 启动后配置现有应用委托的重新打开和终止回调，其余 GLFW 委托方法保持原实现。应用终止先返回取消，由 Go 清理完成后退出事件循环。

## 验证

- `go test -race -tags ci ./...`：完整回归通过，包括关闭不取消应用上下文、任务数据仍可读取、重复隐藏和恢复、显式退出与幂等清理。
- `go vet -tags ci ./...`：通过。
- 隔离原生应用 `tests/nativelifecycle` 使用独立应用身份及临时数据库。红色按钮关闭后仍持续写入后台心跳；重新访问应用时窗口恢复。Command-Q 后生成正常退出标记，最终保存 4438 条心跳记录。
- 原生 Dock 自动化查询曾超时；重新打开验证使用系统应用打开入口，没有取得直接点击 Dock 的验收截图。
- `bash scripts/package.sh`：本机 macOS 应用已重新打包；本次没有操作业务服务器。

证据在 `docs/evidence/macos-window-close/`。可通过 `go build -tags lifecycle_probe -o /tmp/nexshell-lifecycle-probe ./tests/nativelifecycle` 编译隔离原生探针。
