# Language Practice · 码间

面向软件工程师的本地语言学习工作台。每种语言在独立子目录中提供概念学习、代码练习和本地执行服务。

目前已实现 **Go**：14 节中文课程、14 道练习、浏览器代码编辑器、真实编译执行、标准输入、测试模式、格式化和本地学习记录。

## 快速开始

安装 Go 1.22 或更高版本，并确认 `go version` 可用。在本项目根目录执行：

```sh
cd go
go run -buildvcs=false .
```

打开 <http://127.0.0.1:8787>。按 `Ctrl+C` 停止服务。页面不依赖 CDN、npm 安装或互联网连接。

## 目录

```text
language_practice/
├── go/               # Go 课程、网页、本地运行服务和测试
│   ├── web/          # HTML、CSS、JavaScript、课程 JSON
│   ├── docs/         # Go 工作台使用、接口与维护说明
│   └── tests/        # 编辑器行为测试
├── docs/             # 项目文档索引与扩展约定
└── CHANGELOG.md
```

课程内容与运行机制见 [项目文档](docs/README.md) 和 [Go 工作台文档](go/docs/README.md)。

运行的代码拥有当前系统用户的权限；此工作台用于你信任的本地代码，不是隔离沙箱。
