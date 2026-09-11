# 原生文件选择与拖拽上传

## 使用方式

- 将本机文件拖入终端，上传到鼠标所在分屏终端的当前目录；拖到下方“传输”等工具区时使用当前活动终端目录。
- 将文件拖入“文件”列表，上传到文件列表正在浏览的目录。选定目标后切换目录或会话，不会改变已经排队的上传目标。
- 上传开始后显示“传输”页及进度；同名文件继续提供续传、覆盖和取消选项，新文件上传保留排他创建检查。
- 助手输入区域继续接收附件。未连接的终端会提示原因，不再静默忽略拖入的文件。目录仍需先打包为普通文件。
- 上传、下载、背景图片、助手附件、连接导入导出、ZMODEM 文件与目录选择统一走原生选择器。上传和附件支持多选。

macOS 使用 AppKit 的 NSOpenPanel/NSSavePanel，作为主窗口的系统面板呈现。Windows 复用 [Zenity Go 的系统对话框实现](https://github.com/ncruces/zenity)，使用 Explorer 风格选择器。Linux 使用系统 Zenity，需要桌面环境安装对应程序。选择器只返回路径，不提前创建或截断文件；用户取消不报操作失败，任务取消会关闭选择器。

## 修复原因

原拖拽路由只允许在底部“文件”页的文件列表范围内上传，终端和其他工具页都会被忽略。现在根据实际落点选择终端或文件浏览目录，复用原有 SFTP 上传及冲突处理流程。读取终端当前目录和网络传输继续在后台执行。

Fyne 会合并鼠标移动，在文件拖拽回调触发时缓存位置可能还未更新。macOS 和 Windows 在回调时读取系统指针相对内容区的位置，再换算到画布坐标；界面处理通过 `fyne.Do` 回到界面线程。

完整回归另发现 ZMODEM 接收端主动发送准备帧，再次响应发送端启动请求，可能导致重复文件描述和数据帧；提前发送的准备帧还可能被对端初始化清空。现在独立发送端先请求、接收端收到请求再应答；路由器已消费的启动帧直接复用。等待续传校验时只允许有限数量的相同文件描述帧，继续核对前缀校验和，变更的描述或前缀均拒绝续传。

## 验证与范围

证据目录：`docs/evidence/native-file-transfer/`。

- `TestTerminalDropReachesUploadValidation`：修复前终端拖入被静默忽略，修复后进入处理并提示未连接。
- `go test -tags 'ci integration' ./internal/ui -run TestDroppedFilesUploadToIntendedDirectory -count=1`：在隔离 Docker SSH 服务上调用正式拖拽入口，核对落点分屏目录、浏览目录、中文/空格/百分号文件名、文件内容和同名文件保护。
- macOS 原生窗口实测：多选文件、中文路径、保存、选目录、Escape 取消、上下文超时取消；结果见 `macos-native-picker.jsonl`。确认保存路径测试没有创建目标文件。
- 原生窗口验证程序位于 `tests/nativefiles/`，可使用独立应用标识运行，不接触用户连接。macOS 原生面板使用了实际桌面操作；拖拽上传集成验证通过程序调用正式回调，并非 Finder 到正式客户端的完整鼠标拖拽录像。
- `go test -race -tags ci ./...`、`go vet -tags ci ./...`：完整应用竞态回归与静态检查。
- `go test -race ./internal/transfer/...`：原多文件/续传样本与新增重复帧、不同前缀、不同文件描述回归。
- `go test -race -tags integration ./tests/integration -run 'TestLinuxLab/zmodem_lrzsz' -count=3`：真实 lrzsz 连续三次通过；此前失败的握手及续传证据保留在同目录。
- `go test -race -tags integration ./tests/integration -v -count=1`：完整隔离 SSH 集成验证；真实模型测试沿用既有显式开关。
- macOS 打包、签名验证及 Windows 交叉编译。Windows 原生窗口和鼠标拖拽尚未在 Windows 实机验证；Linux 桌面选择器未实机验证。

测试不连接用户服务器，也不调用真实模型。重新打开本轮打包的应用后生效；打包时使用原子替换，不中断已运行客户端的连接。
