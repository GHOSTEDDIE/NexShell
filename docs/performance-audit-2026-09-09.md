# NexShell 卡顿与并发检查（2026-09-09）

> 四项问题现已修复，最新结果见 [修复与验证报告](performance-fix-2026-09-09.md)。下文保留修复前的检查证据。当前代码已包含正式回归测试，请直接运行 `go test -tags ci ./internal/terminal ./internal/ui -run '^TestAudit'`；末尾 overlay 命令仅用于修复前代码，避免与正式测试重名。

## 结论与证据边界

当前工作区代码存在已复现的界面阻塞和关闭标签后对象残留问题，可以解释应用自身卡住、反复使用后内存累积。没有取得用户所述刚才整机卡顿时的进程采样，不能证明该次卡顿由 NexShell 引起。本次只增加诊断报告和测试证据，没有修改产品代码或启动真实服务器操作。

检查开始、运行测试之前的 macOS 进程快照：没有 NexShell/MyAITerminal 进程；Codex Renderer 68.0% CPU、WindowServer 51.5%、WebKit WebContent 25.8%。这些是单次进程 CPU 数据，不能等同于整机 CPU 百分比或历史根因。交换空间使用为 0，Swapins/Swapouts 为 0；这不能单独排除其他内存压力。

## 已复现：P1 网络写入阻塞持有终端全局锁，界面一起等待

位置：`internal/terminal/core.go:53`、`:68`，`internal/terminal/view.go:53`、`:359`；依赖 `third_party/vt/emulator.go` 的 `io.Pipe`、`Paste` 和 `SendText`。

Core.Text 持有 Core.mu 时向无缓冲 io.Pipe 写入；后台 io.Copy 把管道内容送往 SSH。当网络写入阻塞、大段粘贴不能被管道全部读走时，Text 无法释放锁。界面 renderer.paint 调用 Snapshot，等待同一把锁；因此界面刷新也被网络阻塞。Key、Mouse、终端协议响应同样需要关注持锁写管道的路径。这里已证明的是阻塞传播，不是两把锁相互等待的传统死锁。

临时覆盖测试使用实际 Core.Text（64 KiB 粘贴）、io.Copy 和可控阻塞 writer，要求 Snapshot 在 150ms 内返回；连续三次失败。解除阻塞或关闭管道后测试工作协程退出，不遗留运行中的测试进程。此测试没有执行 macOS 图形主循环，界面影响由 renderer.paint 的实际调用链确认。

另一个代码风险：view.go 中 io.Copy 写入失败后只报告错误，没有关闭 Core 输入管道；下一次输入可能永远等不到管道读者。此分支未单独复现。

建议：复用现有输入队列，拆开终端状态锁和可阻塞的发送过程；输入失败须终结输入通道并释放等待者。不要简单去掉锁，否则终端状态会发生并发访问冲突。

## 已复现：P1 关闭标签后工作区仍被 App 引用

位置：`internal/ui/app.go:73`、`:215`，`internal/ui/companion.go:65`，`internal/ui/workspace.go:103`。

新连接追加到 App.workspaces。关闭标签仅调用 workspace.close，取消后台任务、关闭连接与管道；没有从 App.workspaces 删除对象，也没有清空终端、会话和界面引用。整个 View/Core/历史记录对象图因此仍然可达，垃圾回收不能释放它。终端历史虽然每个有 10000 行上限，已关闭工作区数量却会随重复使用增加。

临时覆盖测试通过实际 App.New 和 tabs.CloseIntercept 关闭标签，确认工作区上下文已经取消，再断言 App 不再保留该对象；连续三次失败。该测试证明引用残留，没有测量生产应用每次关闭的具体内存增量。

建议：统一工作区移除入口，同时处理普通关闭、工具栏关闭和应用退出；清理 App 所有权引用，避免在遍历同一切片时删除造成遗漏。

## 代码确认：P2 每秒在界面线程读取全部任务历史

位置：`internal/ui/tasks.go:136`、`:178`，`internal/store/store.go:110`。

watchTasks 每秒通过 fyne.Do 调用 updateTask；后者使用 Events(taskID, 0) 全量读取、解析、拼接历史。即便任务完成且内容没变化，查询和拼接仍照常进行。只有最终 SetText 做了内容比较。历史越长，界面线程工作越多；SQLite 单连接等待也会传到界面线程。

建议：复用 Events 已有的 after 参数在后台增量读取，用事件序号管理显示状态，仅把必要的界面更新提交主线程。必须保留任务切换与 execution_delta 的现有展示语义。本项未用真实长对话测量卡顿时长。

## 基准确认：P2 历史满后每行搬移整个回滚索引

位置：`third_party/vt/scrollback.go:50`，`internal/terminal/core.go:31`。

达到最大行数后，Push 使用 slices.Delete(lines, 0, 1)，每新增一行都移动其余行描述符。它不复制每个历史字符，但每行仍有 O(历史行数) 的搬移开销。

Apple M1 Pro、合成单字符行基准：1000 行上限约 449.8 ns/次，10000 行上限约 4823 ns/次，约 10.7 倍。该结果反映数据结构开销，不能换算成真实终端整体加速倍数。

建议：使用循环缓冲区，保留 Line/Lines/CellAt 的按时间顺序读取约定；回归中文、组合字符、回滚搜索、窗口缩放和备用屏幕。

## 其他检查

- 终端输出刷新有 dirty 标志和 30Hz 定时器；任务轮询每秒、监控每三秒、目录跟随每秒。现有路径没有证实无等待空转的死循环；存在 for 无限循环语法不等于 CPU 空转。
- 跳板机递归先检查 seen，再在递归完成后加主机锁，已避免明显的循环配置递归和跨跳板链锁顺序颠倒。
- Agent 单次模型迭代设置 MaxIterations=100；事件、流读取通过阻塞接口等待。没有以此保证所有外部依赖都可及时取消。
- SFTP 建立、SSH NewSession/Start 等部分阻塞调用发生在取消回调注册之前，超时是否能完整覆盖建立阶段仍需故障服务器测试。未将其列为本次已复现死锁。
- 终端画面每次刷新遍历整屏；修正测试主题后，120×35 的仅 paint 基准约 0.350ms/次、349776 B/次、4236 次分配。未包含真实窗口绘制和显卡开销，不能据此判断整机卡顿。第一轮使用缺少自定义颜色的默认测试主题，产生大量诊断日志，数据作废；保留的是修正后的结果。

## 验证与复现

原有 `go test -race -tags ci ./...` 通过（部分包使用缓存）；`go vet -tags ci ./...` 通过。原有测试通过不覆盖本次新增故障条件。

诊断测试通过 Go overlay 注入，产品源码未改。测试源和输出保存在 `docs/evidence/stall-audit-2026-09-09/`。两项诊断测试以失败断言标记仍待修复的问题，失败是预期诊断结果。

从项目根目录重新生成覆盖配置：

```sh
python3 - <<'PY'
import json
from pathlib import Path
root = Path.cwd()
evidence = root / 'docs/evidence/stall-audit-2026-09-09'
overlay = {'Replace': {
    str(root / 'internal/terminal/audit_stall_test.go'): str(evidence / 'terminal_test.go.txt'),
    str(root / 'internal/ui/audit_lifecycle_test.go'): str(evidence / 'ui_test.go.txt'),
}}
Path('/private/tmp/nexshell-audit-overlay.json').write_text(json.dumps(overlay))
PY
go test -overlay /private/tmp/nexshell-audit-overlay.json -tags ci ./internal/terminal ./internal/ui -run '^TestAudit' -v -count=3 -timeout=30s
go test -overlay /private/tmp/nexshell-audit-overlay.json -tags ci ./internal/terminal -run '^$' -bench '^BenchmarkAudit' -benchtime=200ms -count=1 -timeout=20s
```
