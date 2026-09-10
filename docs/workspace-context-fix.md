# 助手读取会话环境失败修复

## 原因与改动

`workspace_context` 的输入是空结构体，本身没有参数。收到空参数字符串时，原工具适配器仍直接进行 JSON 反序列化，导致 `failed to unmarshal arguments in json` 和 `the input json is empty`，对话在读取会话环境时失败。

使用项目现有 Eino 工具构造选项 `WithUnmarshalArguments`，为这个无参数工具指定解码器：空字符串或纯空白表示没有参数，非空内容仍进行 JSON 解析。不修改模型连接设置，也不改变服务器授权范围；`operate` 等有参数工具继续使用原有解析规则。

## 回归证据

`go test -race ./internal/agent -run '^TestWorkspaceContextEmptyArgumentsConversation$' -count=1 -v` 通过真实 Service、流式模型接口和工具节点回放响应。修改前，空参数稳定重现截图中的完整错误；修改后覆盖：

- 空参数、空白参数、正常 `{}` 都能返回会话环境并继续回复。
- 参数分多段传输，第一段为空、后续段拼成 `{}`，能够正常调用。
- 不完整 JSON 仍报错；有必填参数的 `operate` 仍拒绝空参数。

测试使用临时数据库和模拟模型，没有请求真实提供商或连接服务器。完整回归和静态检查输出见 `docs/evidence/workspace-context-fix/`。旧失败对话可能已留下未完成的工具调用记录，更新后请新建对话验证。
