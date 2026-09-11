# 智能助手展开与收起动画

右侧智能助手增加整体侧滑。展开采用现有 180 毫秒时长，收起采用 120 毫秒时长，两者复用弹窗的 ease-out 曲线。动画服务于面板的空间关系，避免直接出现或消失。

助手面板保持自身宽度与字号，中央工作区的宽度和助手位置由同一个展开比例驱动。展开时中央终端和底部文件区同步收缩，收起时同步展开；切换开始和结束都不会突然改变到最终宽度。空间不足时左右宽度按已有最小尺寸规则平滑插值。拖动分隔条仍直接跟随鼠标。

动画帧直接执行已有布局，避免每帧刷新整个工作台。终端按实际行列变化重排，远端尺寸通知复用已有后台有序队列，网络请求不会在界面线程阻塞动画。

快速反向点击从当前展开比例继续，旧动画帧通过动画序号失效。动画中可缩放主窗口，面板仍以右边缘为基准。过渡期间隐藏右分隔条，落位后恢复。退出时停止动画并使排队帧失效。

沿用“界面动效”开关、Fyne 动效设置及 macOS 的“减少动态效果”设置；禁用时直接切换。没有新增动效依赖。

验证记录位于 `docs/evidence/assistant-sync-motion/`（上一版侧滑证据保留在 `docs/evidence/assistant-motion/`）：

- `TestAssistantSlideInterruptResizeAndReducedMotion`：方向、时长、连续反向点击、过期帧、动画中窗口缩放、禁用动画以及中央模块与助手边界同帧对齐。
- `TestAssistantSlideIgnoresFramesAfterStop`：停止后排队帧不再改变界面。
- 回归原有拖动面板和弹窗动效测试。
- `TestAssistantMotionPreview`：使用实际工作台组件与示例终端内容生成关键帧，供视觉检查。运行时设置 `NEXSHELL_MOTION_EVIDENCE` 输出目录；图像来自 Fyne 测试驱动，不等于原生桌面录屏。
- `BenchmarkAssistantLayoutWithHistory`：Apple M1 Pro、Fyne 测试驱动、两千行历史输出，16 次同步布局平均约 14.1 毫秒；这是该样本的布局耗时，不代表原生窗口的完整帧率。
- `go test -race -tags ci ./...`、`go vet -tags ci ./...`，以及 macOS 打包与签名校验。

本次没有改变终端传输、Agent 和文件操作协议，没有连接真实服务器。Windows/Linux 原生动效手感未进行实机验证。
