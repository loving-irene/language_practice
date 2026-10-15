# Windows 运行环境

语言工作台会在服务启动时检测当前 Windows 用户的工具链，不会自动下载或修改系统环境。选择语言后点击左侧「环境检测」，可以看到当前语言的运行时 / 编译器、路径、版本和缺失状态。安装或修改 PATH 后需要重启服务，再点「环境检测」刷新。

## 最小要求

| 语言 | 工作台需要 | 检测命令 |
| --- | --- | --- |
| Go | Go 1.22+，提供 `go` 和 `gofmt` | `go version` |
| Python | Python 3，提供 `python`、`python3` 或 Windows Python Launcher 的 `py -3` | `python --version` 或 `py -3 --version` |
| C | GCC、Clang 或 MSVC `cl.exe` | `gcc --version`、`clang --version` 或 `cl` |
| C++ | G++、Clang++ 或 MSVC `cl.exe` | `g++ --version`、`clang++ --version` 或 `cl` |
| Rust | `rustc`，建议同时安装 `cargo` 和 `rustfmt` | `rustc --version` |

运行程序还需要对应的编译器 / 运行时在服务进程继承的 `PATH` 中。C 和 C++ 的格式化按钮需要额外的 `clang-format`；Rust 的格式化按钮需要 `rustfmt`。Python 当前使用解释器执行单文件，不依赖第三方包，也不会为你创建虚拟环境。

## 推荐安装顺序

以下命令在 PowerShell 中执行。`winget` 是 Windows 10/11 的常见包管理器；如果本机没有它，请通过 Microsoft Store 的 App Installer 更新，或使用各工具官网安装器。

### Go

```powershell
winget install --id GoLang.Go -e
go version
gofmt -h
```

如果 `go` 仍找不到，重新打开 PowerShell。服务不要求固定安装目录，只要求启动它的终端能够运行 `go version`。

### Python

```powershell
winget install --id Python.Python.3.12 -e
py -3 --version
python --version
```

安装器中勾选 **Add python.exe to PATH**。如果只使用 `py -3` 也可以，工作台会识别 Windows Python Launcher。项目课程只使用 Python 标准库；不要在课程运行时通过 `pip` 安装未知代码。

### C 与 C++：推荐 MSYS2 + MinGW-w64

这条路径同时提供 `gcc` 和 `g++`，适合在普通 PowerShell 中运行：

```powershell
winget install --id MSYS2.MSYS2 -e
```

打开开始菜单中的 **MSYS2 UCRT64**，在 MSYS2 终端执行：

```bash
pacman -Syu
```

如果终端要求关闭并重新打开，照做后再次执行更新，再安装编译器：

```bash
pacman -S --needed mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-clang-tools-extra
```

把 `C:\msys64\ucrt64\bin` 加到 Windows 用户或系统 `Path`，重新打开 PowerShell，然后验证：

```powershell
gcc --version
g++ --version
clang-format --version
```

只需要编译而暂时不需要格式化时，`mingw-w64-ucrt-x86_64-gcc` 已足够。工作台会用兼容性更好的 `-std=c11` 编译 C，用 `-std=c++17` 编译 C++。

### C 与 C++：Visual Studio Build Tools

如果团队标准是 MSVC，可以安装：

```powershell
winget install --id Microsoft.VisualStudio.2022.BuildTools -e
```

安装器中选择 **Desktop development with C++**，保留 MSVC、Windows SDK 和 CMake 工具。然后从开始菜单打开 **x64 Native Tools Command Prompt for VS 2022**，在这个终端里启动工作台并验证：

```bat
cl
```

MSVC 的 `cl.exe` 依赖该开发者终端设置的 `INCLUDE`、`LIB` 和 Windows SDK 环境变量。直接从开发者终端运行工作台最可靠。当前页面识别到 `cl.exe` 时，会使用 MSVC 的 `/EHsc`、`/TC` 和 `/Fe` 参数。

### Rust

推荐使用 rustup 管理工具链：

```powershell
winget install --id Rustlang.Rustup -e
rustup default stable
rustc --version
cargo --version
rustfmt --version
```

如果安装器没有自动更新 PATH，把 `%USERPROFILE%\.cargo\bin` 加入用户 `Path`，重新打开终端。工作台使用 `rustc --edition 2021` 编译单文件；课程不需要 Cargo 项目，但 Cargo 和 rustfmt 方便完整开发。

## 一次性诊断

在启动工作台的同一个 PowerShell / 开发者终端中执行：

```powershell
$commands = 'go','gofmt','python','py','gcc','g++','clang','clang++','clang-format','rustc','cargo','rustfmt'
foreach ($command in $commands) {
  $item = Get-Command $command -ErrorAction SilentlyContinue
  if ($item) { "{0}`t{1}" -f $command, $item.Source } else { "{0}`tNOT_FOUND" -f $command }
}
```

也可以逐个执行版本命令：

```powershell
go version
py -3 --version
gcc --version
g++ --version
rustc --version
```

检测页面只报告路径和版本，不记录密码、令牌或 Cookie。工具链缺失时，课程和代码编辑仍可以浏览；运行按钮会保持禁用，并显示安装提示。

## 常见问题

- **装好了但页面仍显示未安装**：关闭并重新启动 Go 服务。服务只在进程启动时检测工具链。
- **普通 PowerShell 找不到 gcc / g++**：确认 `C:\msys64\ucrt64\bin` 已加入 PATH，使用 `Get-Command gcc` 验证。
- **`cl` 找不到头文件或库**：从 Visual Studio 的开发者命令行启动服务，不要直接双击可执行文件。
- **Python 选择了错误版本**：优先验证 `py -3 --version`。删除或调整 PATH 中的 Python 2 / 旧 Python，重新启动服务。
- **Rust 编译报链接错误**：确认 rustup 的 stable 工具链已安装，并使用 `rustup show` 查看默认工具链。
- **格式化按钮不可用**：运行不依赖格式化器。C/C++ 安装 `clang-format`，Rust 安装 `rustfmt` 后重启服务即可。
- **公司网络限制下载**：使用公司批准的镜像或离线安装包；不要把未验证的第三方二进制放进 PATH。

## 权限和安全

语言代码以启动服务的 Windows 用户权限执行。工作台提供临时目录、超时和输出上限，但不是沙箱；代码仍可能访问文件、网络或启动其他进程。只运行自己信任的代码，不要把服务绑定到公网，也不要通过端口转发给其他用户。
