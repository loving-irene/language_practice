# 项目文档

## 用途与结构

Language Practice 面向有编程基础的软件工程师，提供“理解概念 → 修改代码 → 运行验证”的学习路径。

项目按语言组织。目前只有 `go/` 提供服务，其课程、静态页面、执行器、文档与测试均在该目录内。根目录不启动聚合服务，也没有跨语言依赖。

## 仓库与获取代码

远程仓库为 <https://github.com/loving-irene/language_practice>，主分支为 `main`。

```sh
git clone https://github.com/loving-irene/language_practice.git
cd language_practice
```

源码、课程和文档纳入版本管理；`.artifacts/` 中的本地构建产物、验收截图和运行日志，以及 Go 目录内的可执行文件由 `.gitignore` 排除。

## 快速开始与配置

需要 Go 1.22+。在项目根目录执行：

```sh
cd go
go run -buildvcs=false .
```

默认访问 <http://127.0.0.1:8787>。使用 `go run -buildvcs=false . -port 8888` 更换端口。其他参数、执行限制、环境要求和 API 见 [Go 文档](../go/docs/README.md)。不需要设置项目环境变量。

## 验证

在 `go/` 目录运行：

```sh
go test ./...
go vet ./...
go build -buildvcs=false -o workshop .
```

如果安装了 Node.js 18+，还可以运行不需要额外依赖的前端检查：

```sh
node --test tests/editor.test.mjs
node --check web/app.js
node --check web/editor.js
```

## 扩展语言

新增语言时，使用新的语言子目录作为服务边界，保留该语言自己的课程、运行入口、文档和测试。不要将其他语言的编译器引入 Go 服务，也不要在尚未实现功能时创建空占位项目。共享界面有实际复用需求时再抽取公共资源。

## 常见问题

- **启动后页面打不开**：查看终端是否报告端口占用，改用 `-port` 后访问相应地址。
- **运行报错**：先查看页面中编译 / 运行阶段及错误信息，再检查本机 `go version` 和 `go env GOROOT`。
- **看不到历史记录**：草稿和进度保存在当前浏览器、当前来源的 localStorage 中；更换浏览器、地址或端口会使用另一份记录。
- **更多问题**：参见 [Go 使用与维护说明](../go/docs/README.md#故障排查)。
