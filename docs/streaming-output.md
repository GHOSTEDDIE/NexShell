# 助手流式显示优化

## 发现与测量

原界面在 `watchTasks` 中每秒查询一次任务事件。模型片段已经逐条保存，界面仍需等待下一次轮询，造成按秒集中跳字。实际持久化事件到界面更新的回放测试在 200 ms 门槛下失败；失败记录见 `docs/evidence/streaming/before.txt`。

在 Fyne 视图中回放约 120 段 Markdown，原单次更新基线约 3.07 ms。本次主要优化消息交付调度，保留完整 Markdown 解析，避免分块解析破坏跨段引用、列表或代码块。

## 实现

- 事件或任务状态成功提交后，通过任务订阅唤醒界面读取器。空闲时不再轮询数据库。
- 新消息按约 33 ms 的窗口合并交付，持续输出最高约每秒 30 次更新。
- 通知只保留一个唤醒信号，完整正文仍保存在事件表；界面通过原有序号增量读取，不丢弃输出，也不改变执行证据。
- 保留后台读取与主线程绘制的隔离，最多等待一个界面回调；慢速绘制不会向模型接收链施加通知队列背压。
- 切换任务释放旧订阅，保留任务编号与选择版本检查。退出时关闭订阅并停止调度。
- 停在对话底部时，新内容更新高度后跟随末尾；向上查看历史时保持阅读位置，滚回底部后恢复跟随。

## 验证

`TestStreamingDeliveryLatency` 通过真实 SQLite 事件提交和生产使用的界面交付循环测量，优化后约 36 ms；该值测量到界面回调，不包含显示器扫描时间。

`TestStreamingBurstExactTextAndTaskSwitch` 将 256 个包含中文、组合字符、表情与 Markdown 的片段合并为 6 次更新，完整文字保持一致，完成状态和任务切换正常。

`TestConversationFollowsStreamWithoutStealingScroll` 覆盖长回复自动跟随、上翻历史和恢复跟随。首次回归还发现内容高度需要先刷新才能滚到底，修复前的记录保留于 `scroll-before.txt` 和 `targeted-after.txt`，最终通过记录为 `targeted-final.txt`。

`TestTaskChangesCommitCoalesceAndClose` 和 `TestTaskChangesConcurrentRelease` 覆盖提交后可读、通知合并、跨任务隔离、重复释放、并发订阅释放和关闭后不再订阅。

复现与回归命令：

```sh
go test -race -tags ci ./internal/ui -run 'TestStreaming|TestConversationFollows' -v -count=1
go test -race -tags ci ./internal/store -run TestTaskChanges -v -count=1
go test -race -tags ci ./...
go vet -tags ci ./...
go test -race -tags integration ./tests/integration -v -count=1
```

全部测量及构建输出位于 `docs/evidence/streaming/`。本次使用确定性回放及隔离 SSH 夹具，不需要重新调用真实模型服务。

最终长回复单次排版约 3.16 ms（`render-final.txt`）。完整并发回归、静态检查、隔离 Linux 原有正确样本及 macOS arm64、Linux arm64、Windows amd64 构建均通过；真实模型验收本次未重复调用。
