# Bajiu DB · 捌玖

基于 Go + Wails + Vue 3 的跨平台数据库桌面工具。当前版本为 **0.1.0：MySQL 连接管理**。

## 已实现

- 新建、编辑、删除和复制连接配置。
- 名称与主机搜索，按本地、开发、测试、生产分组。
- 测试真实 MySQL 登录与数据库访问，显示服务器版本及连接耗时。
- 打开、断开连接，读取和刷新当前账户可访问的数据库列表。
- 可选默认数据库、5–30 秒超时，以及验证服务器证书的 TLS 模式。
- 可选 CA 证书和证书主机名；不提供跳过证书验证的模式。
- 本地保存配置，密码使用操作系统凭据库保存。
- 复制配置不会复制密码；离开编辑页面时提示未保存的修改。
- 配置损坏时保留原文件，并停止保存操作。

当前没有 SQL 编辑、表数据编辑、SSH 隧道和数据库迁移功能。

## Windows 运行

打开 `build/bin/BajiuDB.exe`，或解压发布 ZIP 后打开 `BajiuDB.exe`。

程序使用系统 WebView2。此依赖不包含在便携 ZIP 中；本机没有运行时时，需要安装 Microsoft WebView2 Runtime。Go 和 Node.js 只在开发时需要，用户运行程序时不需要安装。

新建连接时填写名称、主机、端口和 MySQL 账户，先点击“测试连接”，再保存或保存并连接。MySQL 账户与操作系统账户分别管理，Bajiu DB 不会创建数据库账户。

密码可以不保存，仅用于本次编辑或连接操作；切换连接或重启程序后需要重新输入。编辑已有连接时，密码栏留空表示继续使用已存密码。移除已存密码后，需要保存配置才会生效。

保存已有连接的修改会断开该连接的旧会话。数据库列表由连接时或手动刷新时读取；保持一个会话不代表持续监测服务器可达性，刷新列表可以再次验证。

## 数据位置

配置位于 `os.UserConfigDir()/BajiuDB/connections.json`：

- Windows：`%APPDATA%/BajiuDB/connections.json`
- macOS：`~/Library/Application Support/BajiuDB/connections.json`
- Linux：`$XDG_CONFIG_HOME/BajiuDB/connections.json`，未设置时通常为 `~/.config/BajiuDB/connections.json`

配置中只包含连接参数和凭据引用，不包含明文密码。凭据服务名为 `BajiuDB.connections`；Windows 使用 Credential Manager，macOS 使用 Keychain，Linux 使用 Secret Service。系统凭据库不可用时可以不记住密码，不会退回明文存储。配置文件可以备份，但密码不会随 JSON 文件跨机器迁移。

应用不提供云同步、遥测或自动更新功能。

## 开发与构建

当前锁定 Wails **v2.11.0**，已在 Go **1.25.5** 与 Node.js **22.20.0** 下验证。Go 模块与 npm 依赖均有锁文件。修改框架版本时应重新运行构建与集成测试。

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0
cd frontend
npm ci
cd ..
wails doctor
wails dev
```

生产构建：

```sh
wails build -trimpath -ldflags "-s -w"
```

Windows 也可以运行 `scripts/build.ps1`。该脚本使用已安装的 Wails，或通过固定版本的 `go run` 执行构建。

Windows、macOS 和 Linux 分别在相应平台上构建。macOS 需要 Xcode 工具链；Linux 需要 GTK 3、WebKitGTK 和 C 编译工具，使用 WebKitGTK 4.1 时按 Wails v2 文档添加 `-tags webkit2_41`。密码保存依赖该系统可用的凭据服务。macOS/Linux 尚未在本次环境中实际构建或运行。

## 验证

```sh
go test ./...
go vet ./...
cd frontend
npm test
npm run build
npm audit
```

可选集成测试使用调用者提供的独立 MySQL 测试实例，不读取本机已有应用的账户或密码。测试服务器需含 `bajiu_test_db` 数据库，使用不受信任的测试证书，并为测试账户授予该数据库的读取权限。

PowerShell 环境变量示例，以下凭据仅供测试：

```powershell
$env:BAJIU_MYSQL_TEST_ADDR = '127.0.0.1:33179'
$env:BAJIU_MYSQL_TEST_USER = 'bajiu_test'
$env:BAJIU_MYSQL_TEST_PASSWORD = 'bajiu-test-only-2026'
$env:BAJIU_TEST_KEYRING = '1'
go test ./internal/... -count=1 -v
```

集成测试检查真实服务器登录、元数据读取、错误密码、TLS 验证失败，以及系统凭据库的存取和清理。未设置变量时跳过相关测试。

## 代码结构

- `internal/connections`：配置、凭据、MySQL 访问、会话生命周期和测试。
- `app.go`：桌面方法桥接与本机文件选择。
- `main.go`：窗口和桌面生命周期。
- `frontend/src`：中文界面、交互和界面流程测试。
- `build`：图标、平台资源与本机编译产物。
- `THIRD_PARTY_NOTICES.md`、`licenses/`：第三方声明及许可证。

第三方组件按各自许可证分发；应用自身的发布许可由项目所有者决定。MySQL 驱动采用 MPL-2.0，对应源码和获取方式见第三方声明。
