# 安装与配置

[English](installation.md) · **简体中文** · [文档目录](README.zh-CN.md)

## Windows x64

打开 [GitHub 发布页](https://github.com/iam671/BajiuDB/releases)或 [Gitee 发布页](https://gitee.com/sixeventy-one/bajiu-db/releases)。若已发布附件，下载 `BajiuDB-0.0.1-windows-x64.zip` 及 `.sha256` 校验文件，解压后运行 `BajiuDB.exe`。若尚无二进制附件，请[从源码构建](development.zh-CN.md)。

程序使用系统 Microsoft WebView2 Runtime，不随 ZIP 打包。缺少运行时时请从 [Microsoft](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) 安装。运行成品无需安装 Go 或 Node.js，当前界面语言为简体中文。

PowerShell 中可将 ZIP 的 SHA-256 与发布的校验文件比较：

```powershell
Get-FileHash -Algorithm SHA256 ./BajiuDB-0.0.1-windows-x64.zip
Get-Content ./BajiuDB-0.0.1-windows-x64.sha256
```

更新时先保存或放弃未保存内容，关闭旧程序后再打开新包。构建产物与用户连接配置分开存放。

## 其它平台

Go、Wails、Vue 为 Windows/macOS/Linux 构建提供基础。当前发布流程生成 Windows x64 包，macOS/Linux 桌面构建及运行尚待验证。需在目标系统安装 Wails 原生依赖并构建，不能直接运行 Windows EXE。详见[开发指南](development.zh-CN.md)。

## 连接设置

点击“连接”先选择引擎，再填写连接名称及对应参数，测试后保存。已保存连接的数据库类型固定。SQLite 打开已有普通文件，不静默新建数据库。各引擎配置见[数据库连接](database-engines.zh-CN.md)。

记住密码时使用系统凭据库。编辑已有连接时密码留空表示沿用原凭据；显式移除已存密码后，保存配置才生效。不记住密码时，输入的密码仅在当前应用进程内存中保存。

## 配置与凭据位置

连接和分组配置位于 `os.UserConfigDir()/BajiuDB/connections.json`：

| 系统 | 常见路径 | 凭据服务 |
| --- | --- | --- |
| Windows | `%APPDATA%/BajiuDB/connections.json` | Credential Manager |
| macOS | `~/Library/Application Support/BajiuDB/connections.json` | Keychain |
| Linux | `$XDG_CONFIG_HOME/BajiuDB/connections.json`，通常为 `~/.config/BajiuDB/connections.json` | Secret Service |

凭据服务名为 `BajiuDB.connections`。JSON 保存连接参数及凭据引用，不保存明文密码；备份 JSON 不会把密码迁移到另一台机器。系统凭据库不可用时可不记住密码连接，程序不会退回明文保存。

地址、连接名和用户名仍属于配置内容，不要把个人配置文件提交到公共仓库或问题反馈中。程序不提供内置云同步、遥测或自动更新。

## 文件格式依赖

TXT/CSV/JSON/XML/XLSX/SQLite DB 使用内置读写实现；XLS/MDB/ACCDB/DBF 需要 Windows x64 ACE，ODBC 需要匹配的系统驱动。驱动不随应用分发或自动安装，使用前请查看[格式指南](data-transfer.zh-CN.md)。
