# NexShell

面向运维人员的全 Go SSH 桌面客户端，结合 Eino DeepAgent 实现按任务授权的服务器运维助手。项目采用 Apache-2.0，当前处于 **0.1 开发阶段**。

## 运行

需要 Go 1.25、C 编译器及对应平台的图形开发依赖：

```sh
go run ./cmd/nexshell
```

独立测试配置目录：

```sh
go run ./cmd/nexshell -data-dir /tmp/nexshell-demo
```

原生打包：

```sh
bash scripts/package.sh
```

macOS 使用 Xcode Command Line Tools；Windows 构建需要 GCC/MinGW；Linux 构建需要 OpenGL、X11、Wayland 和 xkbcommon 开发库。当前已生成 macOS arm64、Linux arm64、Windows amd64 二进制。三端构建流程位于 `.github/workflows/ci.yml`。构建成功与目标系统实机验收分开记录，详见 [验证记录](docs/validation.md)。

## 使用

1. 添加服务器，填写地址、账号和认证方式。支持密码、私钥、系统 SSH Agent 与交互认证；跳板机从已保存的主机中选择。
2. 首次连接核对服务器指纹。服务器密钥变化会阻断连接，不会自动覆盖旧记录。
3. 选择主机后连接，使用多标签或分屏终端，以及同屏文件、监控、传输和网络诊断。
4. 在“模型设置”中配置 OpenAI 兼容接口或 Ollama 的地址与模型。密码和模型密钥保存在系统凭据库，JSON 导出不包含这些凭据。
5. 新建运维任务，选择目标服务器，填写目标和允许操作的资源。读取文件、修改文件、重启服务及安装软件包可以逐类授权；资源按精确路径或名称匹配。
6. 任意 Shell 命令会显示具体请求供确认。任务面板记录计划、执行结果和验证证据。停止、断线和应用退出不会触发变更命令自动重放。

助手支持复制文件后粘贴到对话框、输入本地路径、选择或拖入附件，读取文本参与对话，将部署包上传到授权服务器并独立核对文件校验值。见 [本地文件与部署包](docs/local-deployment.md)。

命令历史只记录命令栏中勾选“记住命令”的输入，不记录终端逐键输入。快捷命令使用 `{{参数名}}` 占位，插入时填写参数；参数会作为单个 Shell 参数引用，不要在占位符外重复添加引号。

在远端执行 `rz` 会打开本地文件选择器；执行 `sz 文件` 会选择下载目录。ZMODEM 协议在 Go 内实现，客户端无需安装 `lrzsz`。传输支持二进制数据与多文件协议、续传校验、取消；当前文件选择器一次选择一个上传文件。服务端仍需提供 `rz/sz`。

文件保存会校验读取时的哈希、保留备份，并要求服务器支持原子覆盖。SFTP 无法锁住其他客户端，因此哈希检查与远端替换之间仍存在竞争窗口；不要把它视为跨客户端事务。续传验证已有文件前缀，完整传输完成后再次校验文件。

## 当前边界

- Linux 监控与常见运维操作已实现；软件包安装适配 apt-get/dnf，服务管理使用 systemctl，权限按 SSH 账号实际权限执行。
- Agent 的取消会发送终止信号；无法确认远端退出时记录为“结果未知”。应用重启后授权失效，需要用户恢复；有未知执行的任务要求先人工核实并用新任务继续。
- 自动任务完成依赖末次只读证据检查；证据检查不是对任意运维目标的形式化证明，生产使用仍需要业务验收。
- 本地 Skills 可放置于应用数据目录的 `skills/<名称>/SKILL.md`。内置诊断与配置变更手册随应用提供。
- 助手支持单层子 Agent 委派、标准 Skills 和按服务器隔离的自动记忆，见 [DeepAgent 与迁移说明](docs/deep-agent.md)。本版本无远程桌面、云同步、常驻执行服务或外部 MCP 工具接入。
- Windows/Linux 原生安装包与三端输入法候选窗口需要在对应系统验收；当前本地构建证据以 macOS arm64 为准。

## 开发与测试

```sh
go test -race -tags ci ./...
go vet -tags ci ./...
docker build -t nexshell-test-sshd:local tests/fixture
go test -race -tags integration ./tests/integration -v -count=1
```

集成测试自动创建仅绑定本机回环地址的独立 SSH 容器，使用临时密码，不挂载宿主目录，结束后删除测试容器。测试包含真实 SSH、SFTP、端口转发、lrzsz 双向互通和 Eino 授权恢复。模型确定性测试与真实模型效果验收分别记录。

终端依赖存在一个有记录的局部修复，修改后还需执行：

```sh
cd third_party/vt
go test ./...
```

更多内容：[架构与调研](docs/architecture.md) · [验证记录](docs/validation.md) · [贡献指南](CONTRIBUTING.md)。
