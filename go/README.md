# Go 语言工作台

一套无需前端构建的多语言本地学习服务。页面目前包含 Go、Python、C、C++、Rust 五种语言。

## 启动

先安装 Go 1.22+ 作为服务运行时；要运行其他语言，按 [Windows 环境文档](docs/windows-environment.md) 安装对应工具链。在本目录执行：

```sh
go run -buildvcs=false .
```

访问 <http://127.0.0.1:8787>。可选参数：

```sh
go run -buildvcs=false . -port 8888 -timeout 5s -build-timeout 60s
```

## 包含什么

- 概念学习：Go 14 节，Python / C / C++ / Rust 各 4 节中文课程。
- 代码实验室：语法着色、行号、缩进、格式化、标准输入、运行结果、练习提示和预期输出。
- 本地执行：按语言选择解释器或编译器；Go 支持单文件程序和 `_test.go` 测试，其他语言支持单文件程序。
- 环境检测：检测 Python、GCC、G++、Clang、MSVC、rustc、rustfmt 等工具，并提供 Windows 安装说明。
- 浏览器记录：独立的示例 / 练习草稿、课程进度、最近 8 次运行输出。

代码以本机用户权限运行。仅运行自己信任的代码，不应将该服务代理到公网。

完整的运行机制、API、课程维护与验证方式见 [docs/README.md](docs/README.md)。
