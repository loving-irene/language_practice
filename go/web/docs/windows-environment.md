# Windows 运行环境

本地语言工作台只会检测工具链，不会自动安装。安装或修改 Windows `PATH` 后，重启 Go 服务，再点击「环境检测」。

## 快速安装

在 PowerShell 中执行：

```powershell
winget install --id GoLang.Go -e
winget install --id Python.Python.3.12 -e
winget install --id MSYS2.MSYS2 -e
winget install --id Rustlang.Rustup -e
```

MSYS2 安装后打开 **MSYS2 UCRT64**，执行：

```bash
pacman -Syu
pacman -S --needed mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-clang-tools-extra
```

把 `C:\msys64\ucrt64\bin` 加到 Windows `Path`。Rust 安装后执行 `rustup default stable`，必要时把 `%USERPROFILE%\.cargo\bin` 加入 `Path`。如果使用 MSVC，安装 Visual Studio Build Tools 的 **Desktop development with C++**，并从 **x64 Native Tools Command Prompt for VS 2022** 启动工作台。

## 验证

```powershell
go version
gofmt -h
py -3 --version
gcc --version
g++ --version
rustc --version
cargo --version
rustfmt --version
```

工作台需要 Python 3、GCC/Clang/MSVC、Rust 以及 Go 在启动服务进程的 PATH 中。C/C++ 的格式化需要 `clang-format`，Rust 的格式化需要 `rustfmt`。缺失工具链只会禁用对应语言的运行按钮，不影响其他课程。

完整的 MSYS2、MSVC、诊断脚本和安全边界见项目中的 `go/docs/windows-environment.md`。
