# Go 工作台使用与维护

## 项目结构

| 文件 / 目录 | 职责 |
| --- | --- |
| `main.go` | 启动参数、仅本机 HTTP 服务、静态资源嵌入、请求校验和格式化 |
| `runner.go` | 临时工作目录、编译、程序执行、输入输出和资源限制 |
| `process_windows.go` / `process_unix.go` | Windows 进程树 / Unix 进程组的取消处理 |
| `web/index.html` / `web/styles.css` | 响应式中文工作台 |
| `web/app.js` / `web/editor.js` | 课程导航、草稿、进度、编辑器与 API 交互 |
| `web/lessons.json` | 14 节课程的讲解、示例、练习、提示、预期输出与参考解法 |
| `main_test.go` | 服务、执行器、错误路径以及全部课程代码的验证 |
| `tests/editor.test.mjs` | 转义、缩进、着色和输出核对测试 |

## 环境与快速开始

支持 Windows、Linux、macOS。需要 Go 1.22 或更高版本，且 `go` 在启动服务的进程 PATH 中。浏览器需支持 ES modules、dialog、fetch 和 localStorage；推荐近期版本的 Chrome、Edge、Firefox 或 Safari。

在 `language_practice/go` 目录执行：

```sh
go version
go run -buildvcs=false .
```

访问 <http://127.0.0.1:8787>。终端按 `Ctrl+C` 关闭服务。浏览器页面由 Go 二进制嵌入，不需要 Node.js、npm 或 CDN。修改 `web/` 后需要重启服务；使用构建产物时需要重新构建。

启动与构建命令使用 `-buildvcs=false`，关闭与本地练习无关的 Git 元数据写入。这让没有 Git 仓库或无法读取父目录 Git 状态的独立项目也能构建，不影响代码检查和测试。

可选构建方式：

```sh
go build -buildvcs=false -o workshop .
./workshop
```

Windows PowerShell 使用：

```powershell
go build -buildvcs=false -o workshop.exe .
.\workshop.exe
```

构建后的服务仍需要本机 Go 工具链来编译用户练习，不能仅分发可执行文件就省略 Go 安装。

## 配置

配置使用命令行参数，不新增项目环境变量：

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `-port` | `8787` | 1–65535，仅绑定 `127.0.0.1` |
| `-timeout` | `5s` | 每次程序执行的时间上限，必须大于零 |
| `-build-timeout` | `60s` | 单独的编译上限，首次编译标准库可能耗时更长 |

```sh
go run -buildvcs=false . -port 8888 -timeout 10s -build-timeout 90s
```

标准库编译缓存由 Go 的正常环境配置管理。执行器仅转发运行和查找工具所需的环境变量（PATH、系统目录、临时目录、用户目录、GOROOT、GOCACHE 和 GOPATH 等），不整份转发服务进程的环境。它明确设置 `GOPROXY=off`、`GOSUMDB=off`、`GOTOOLCHAIN=local`、`GOWORK=off`、`GOENV=off`、`GOFLAGS=`、`CGO_ENABLED=0` 和 `GOMAXPROCS=2`。

只支持当前 Go 安装中的标准库，不下载外部依赖、不支持多文件模块或 CGO。上述环境设置不限制用户程序自己的网络或文件访问。

## 学习与练习

1. 从左侧选择课程，阅读概念、代码片段与工程师笔记。
2. 点击「运行代码」，查看真实 stdout / stderr 和退出码；用「格式化」整理代码。
3. 切换「动手练习」，根据题意修改起始代码；可以查看提示和预期输出。
4. 练习成功执行且输出匹配后自动记录进度，也可手动「标记为已掌握」。
5. 「代码实验室」提供更宽的编辑区；编辑内容仍与当前课程、示例 / 练习模式关联。

普通程序的练习会归一化 CRLF 和末尾空白，再对照预期输出。测试课程要求进程成功并包含具名测试通过的结果。这是学习反馈，不是防作弊判题器，不保证任意算法或所有边界都正确。参考解法保存在课程 JSON 中用于维护验证，界面通过提示引导完成练习。

标准输入在运行前整体送入程序，随后关闭，不是持续交互终端。用 `fmt.Scan`、`fmt.Fscan` 或 `bufio.Scanner` 读取输入。长驻服务、无限等待输入或超过时间限制的程序会被停止。

测试模式把编辑内容写入 `main_test.go`，执行 `go test -c` 生成测试二进制，再执行带 `-test.v` 和 `-test.timeout` 的测试程序。正常模式写入 `main.go` 并用 `go build` 编译。页面中的测试示例可直接运行。

### 快捷键

| 操作 | Windows / Linux | macOS |
| --- | --- | --- |
| 运行代码 | Ctrl+Enter | ⌘+Enter |
| 格式化代码 | Ctrl+Shift+F | ⌘+Shift+F |
| 搜索课程 | Ctrl+K | ⌘+K |
| 增加 / 减少缩进 | Tab / Shift+Tab | Tab / Shift+Tab |
| 将焦点移出编辑器 | 先 Esc，再 Tab | 先 Esc，再 Tab |

## 数据保存

代码、标准输入、运行模式、当前课程、掌握标记和最近 8 次输出保存在 localStorage 的 `language-practice.go.v1` 项下。示例和练习拥有独立草稿。数据不上传第三方服务，也不存入服务端数据库。

保存按浏览器来源隔离，`localhost` 与 `127.0.0.1`、不同端口和不同浏览器各有记录。清除浏览器数据会删除记录。浏览器禁止存储或配额不足时，界面会提示保存失败，此时应自行复制代码。重置按钮仅重置当前课程当前模式的代码与标准输入，并要求确认。

## 执行与访问边界

- 每个请求使用独立临时目录，执行结束后删除目录。编译和运行分别设置超时。
- 同时允许 2 个任务；第三个请求返回 HTTP 429。取消浏览器请求会取消对应执行。
- 请求体上限 128 KiB，源代码上限 64 KiB，标准输入上限 16 KiB。
- stdout 和 stderr 合并，最多保留前 64 KiB；超过部分丢弃并返回 `truncated: true`，程序仍受运行超时控制。
- 只接受本机 Host，校验 Origin、Sec-Fetch-Site 与每次启动随机生成的会话令牌。POST 必须使用 JSON。页面设置 CSP，代码和输出以文本呈现。
- 超时与主动取消在 Windows 尝试终止进程树，在 Unix 终止进程组。这不是容器隔离：没有内存、磁盘或权限沙箱，恶意程序仍可能访问本机文件、网络或创建脱离管理的进程。只用于受信任代码，不适合公网或多用户部署。

## HTTP API

所有 API 同源访问。`GET /api/config` 返回当前会话 token、Go 工具链版本、超时与输入限制。Token 只用于本地请求校验，不应写入日志。

`POST /api/run` 和 `POST /api/format` 使用 `Content-Type: application/json`，并在 `X-Practice-Token` 中传入配置接口返回的 token。

运行请求：

```json
{
  "code": "package main\nfunc main() { println(42) }",
  "input": "",
  "mode": "run"
}
```

`mode` 为 `run` 或 `test`，省略时默认为 `run`。格式化只需 `code` 字段，成功返回 `{"code":"格式化后的源码"}`。

运行响应字段：

| 字段 | 含义 |
| --- | --- |
| `success` | 编译、执行成功且退出码为 0 |
| `output` | 合并后的 stdout / stderr |
| `phase` | 结果发生于 `build`、`run` 或 `test` |
| `exitCode` | 退出码；无法取得或任务取消时可能为 -1 |
| `durationMs` | 编译和执行的总耗时 |
| `timedOut` | 是否触发编译或执行超时 |
| `truncated` | 是否触发输出上限 |

编译错误和程序错误使用 HTTP 200 加 `success: false` 表达。请求格式错误为 400、访问校验失败为 403、请求过大为 413、非 JSON 为 415、格式化语法错误为 422、任务满载为 429、执行器自身故障为 500。错误响应为 `{"error":"可读错误"}`。不接受未知 JSON 字段或多个 JSON 值。

## 测试与课程维护

```sh
go test -v ./...
go vet ./...
go build -buildvcs=false -o workshop .
```

Go 测试会真实编译运行程序，验证正常输出、标准输入、stderr、编译失败、panic、测试成功 / 失败、执行 / 编译超时、输出截断、临时目录清理、环境过滤、接口校验和并发上限。`TestLessonPrograms` 执行所有课程示例及练习参考解法。

前端行为测试需要 Node.js 18+，无需 `npm install`：

```sh
node --test tests/editor.test.mjs
node --check web/app.js
node --check web/editor.js
```

修改课程时维护 `web/lessons.json` 中唯一的 `id`、分组、概念讲解、示例、练习及预期输出。保持示例只依赖标准库，结果可重复；测试模式允许测试框架自己的时间信息。修改后执行 `go test -run TestLessonPrograms -v`。再在浏览器检查课程导航、示例 / 练习草稿隔离、手机布局和错误反馈。

## 故障排查

- **go 不在 PATH**：安装工具链后重新打开终端。服务启动时会检查工具是否存在。
- **package ... is not in std**：确认 `go env GOROOT` 指向完整安装，当前进程有权读取 Go 的 src、pkg 和编译缓存；受限运行环境可能需要额外文件访问权限。
- **address already in use**：使用 `-port 8888` 等空闲端口，访问对应 URL。
- **会话失效**：服务重启后 token 会变化，点击右上角连接状态重新连接，或刷新页面。
- **首次编译超时**：先检查 Go 安装和磁盘权限，确有需要时提高 `-build-timeout`。
- **执行超时**：检查无限循环、channel 阻塞、读取标准输入或启动长期服务等情况；为有界但耗时的练习适度提高 `-timeout`。
- **页面没有更新**：静态文件被嵌入二进制，需要重启或重新构建。服务响应不缓存页面资源。
- **编辑器功能范围**：轻量文本编辑器提供着色和缩进，不提供语言服务器、智能补全或断点调试。复杂项目仍适合在 IDE 中完成。
