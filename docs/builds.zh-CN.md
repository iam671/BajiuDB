# 发布构建

[English](builds.md) · **简体中文** · [文档目录](README.zh-CN.md)

当前版本 **0.0.1** 提供以下构建目标。交叉编译成功不等于已完成目标系统的桌面运行验证。

| 平台 | CPU | 压缩包 | 状态 |
| --- | --- | --- | --- |
| Windows | x64 | `BajiuDB-0.0.1-windows-x64.zip` | 沿用 Windows 原生生产构建 |
| Linux | x64 | `BajiuDB-0.0.1-linux-x64.tar.gz` | 实验性 purego 后端交叉编译，未在 Linux 启动 |
| Linux | ARM64 | `BajiuDB-0.0.1-linux-arm64.tar.gz` | 实验性 purego 后端交叉编译，未在 Linux 启动 |
| macOS | Intel x64 | `BajiuDB-0.0.1-macos-x64.zip` | `.app` 包，purego 交叉编译；无 Developer ID 签名/公证，未在 Mac 运行 |
| macOS | Apple Silicon | `BajiuDB-0.0.1-macos-arm64.zip` | `.app` 包，purego 交叉编译；无 Developer ID 签名/公证，未在 Mac 运行 |

每个压缩包均附 `.sha256` 文件，包含中英文 README、Markdown 指南、截图、中转脚本、项目及第三方许可材料。`SHA256SUMS.txt` 校验可执行文件；Unix 包还附 `BUILD-INFO.json` 和中英文 `README-platform.md`。macOS ZIP 与 Linux tar 都保留可执行权限。

## 运行条件

- **Windows：** Microsoft WebView2 Runtime。
- **Linux：** 图形桌面、GTK4、WebKitGTK 6.0，包内不附运行库。Ubuntu 24.04/Debian 13 的包名为 `libgtk-4-1 libwebkitgtk-6.0-4`，其它发行版安装对应库。此 GTK4 包不以 Ubuntu 22.04/Debian 12 为目标。记住密码需要已解锁的 Secret Service，如 GNOME Keyring。解压后运行 `./BajiuDB`。
- **macOS：** 应用包部署目标为 macOS 12+。解压 ZIP 后将 `Bajiu DB.app` 移入“应用程序”，按 CPU 选择版本。此为未做 Apple Developer ID 签名和公证的测试包，macOS 可能阻止打开。签名、公证及启动验证仍需 Mac 和对应签名身份。

应用与系统驱动格式的条件不同：ACE/ODBC 格式仍限 Windows，详见[数据导入导出](data-transfer.zh-CN.md)。发布包不包含私人凭据或用户连接配置。

Wails 3.1.0 提供 `CGO_ENABLED=0` 与 `-tags purego,production` 的实验性交叉构建后端，系统运行库及签名仍需单独满足。参考 [Wails 发布说明](https://v3.wails.io/blog/wails-v3-1-released/)和 [Linux 打包说明](https://v3.wails.io/guides/build/linux/)。

## Windows 一次构建全部目标

准备项目 Go/Node 工具链与 **Python 3.11+**，在 Windows PowerShell 运行：

```powershell
./scripts/package-all.ps1
```

脚本先调用既有 Windows 构建/打包流程，生成前端并收集许可，再以 `CGO_ENABLED=0` 和 `production,purego` 构建四个 Linux/macOS 二进制，检查格式/CPU、包内二进制哈希及权限。该路径不依赖 C 编译器、Docker 或远程构建服务；Python 仅为打包工具，不是应用运行依赖。

需要复用 `.local/cross-build` 里已经编译的文件时可加 `-SkipCrossBuild`。`-SkipWindowsBuild` 会跳过 Windows/前端准备，仅在已有产物和许可材料都最新时使用。两个选项只复用文件，不证明二进制与后续源码修改一致。

新包暂存并验证成功后，仅替换相同版本的固定产物名称，不清理其它版本、连接配置或凭据。桌面运行、代码签名与数据库兼容性独立于编译/归档检查，具体见[验证记录](../VALIDATION.md)。
