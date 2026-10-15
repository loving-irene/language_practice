# Language Practice · 码间

面向软件工程师的本地语言学习工作台。每种语言在独立子目录中提供概念学习、代码练习和本地执行服务。

目前已实现 **Go、Python、C、C++、Rust**：每种语言有独立课程、浏览器代码编辑器、本地运行和学习记录。Go 支持单文件测试；其他语言先提供单文件程序练习。

## 快速开始

在 Windows 上先按 [环境依赖文档](go/docs/windows-environment.md) 安装所需工具链。在本项目根目录执行：

```sh
cd go
go run -buildvcs=false .
```

打开 <http://127.0.0.1:8787>。按 `Ctrl+C` 停止服务。页面不依赖 CDN、npm 安装或互联网连接。

## 目录

```text
language_practice/
├── go/               # 多语言课程、网页、本地运行服务和测试
│   ├── web/          # HTML、CSS、JavaScript、课程 JSON
│   ├── docs/         # Go 工作台使用、接口与维护说明
│   └── tests/        # 编辑器行为测试
├── docs/             # 项目文档索引与扩展约定
└── CHANGELOG.md
```

课程内容、工具链检测和运行机制见 [项目文档](docs/README.md) 和 [Go 工作台文档](go/docs/README.md)。

运行的代码拥有当前系统用户的权限；此工作台用于你信任的本地代码，不是隔离沙箱。
