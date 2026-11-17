# Bajiu DB · 捌玖

基于 Go + Wails + Vue 3 的跨平台数据库桌面工具。当前版本为 **0.0.1：桌面工作区与 MySQL 连接管理**。

## 已实现

- 新建、编辑、删除和复制连接配置。
- 右键“我的连接”可新建连接、新建组、关闭所有连接。右键已有连接选择“移至分组…”可调整归属，也可移回未分组；在组上右键可新建组内连接或重命名组。
- 自定义分组与连接配置一起保存，空组及连接归属重启后保留。环境标签与自定义分组分别管理。
- 连接可直接拖入分组，也可拖到“我的连接”、左侧空白处或拖拽时显示的“移回我的连接”区域来移出分组。保存失败时保留原归属；拖拽不会关闭会话或清空当前对象列表。
- 不显示底部 Toast；成功操作直接反映在界面状态中，错误在操作位置或左侧导航中显示。
- 新建组、重命名组直接在左侧树中输入，Enter 保存、Esc 取消，也可点击其他位置提交；保存失败会保留输入并就地提示。右键组选择“删除组”，确认后组内连接移回“我的连接”，配置、密码和已打开的会话保留。
- 菜单与大图标工具栏、左侧连接树、中间对象列表、右侧信息窗格及底部状态栏。
- 左右窗格可拖动、可用方向键调整宽度；支持隐藏信息窗格、详细信息/图标视图及本机布局记忆。
- 独立连接弹窗，含常规、高级、数据库、SSL、SSH、HTTP 分页；可自由拖动标题栏，支持键盘焦点约束和未保存确认。
- 名称与主机搜索，按本地、开发、测试、生产分组。
- 测试真实 MySQL 登录与数据库访问，显示服务器版本及连接耗时。
- 打开、断开连接，读取和刷新当前账户可访问的数据库列表。
- 浏览数据库中的表、视图、存储过程/函数、事件元数据，并显示所选对象的详细信息。
- 连接与数据库节点仅在可展开时显示箭头；数据库成功打开前为灰色，成功读取对象列表后为绿色，断开连接时重置。
- 可选默认数据库、5–30 秒超时，以及验证服务器证书的 TLS 模式。
- 可选 CA 证书和证书主机名；不提供跳过证书验证的模式。
- 本地保存配置，MySQL 密码、SSH 密码/私钥口令和 HTTP 令牌使用操作系统凭据库保存；私钥文件仍由用户管理。
- 复制配置不会复制密码；关闭连接弹窗时提示未保存的修改。
- 配置损坏时保留原文件，并停止保存操作。

当前没有 SQL 编辑、表数据编辑和数据库迁移功能。支持 SSH 密码/私钥隧道，以及配套 PHP 脚本的网站 URL 中转，部署说明见 `tunnel/README.md`。

## Windows 运行

打开 `build/bin/BajiuDB.exe`，或解压发布 ZIP 后打开 `BajiuDB.exe`。

程序使用系统 WebView2。此依赖不包含在便携 ZIP 中；本机没有运行时时，需要安装 Microsoft WebView2 Runtime。Go 和 Node.js 只在开发时需要，用户运行程序时不需要安装。

点击工具栏“连接”打开新建连接弹窗，填写名称、主机、端口和 MySQL 账户，点击“测试连接”，再点击“确定”保存。双击左侧连接打开服务器，单击数据库显示表列表；双击数据库展开/收起分类，可选择“表、视图、函数、事件”。单击连接可返回数据库列表。单击中间对象可查看右侧详细信息。MySQL 账户与操作系统账户分别管理，Bajiu DB 不会创建数据库账户。

密码可以不保存，保存配置时输入的密码只留在本次进程内存中，重启后需要重新输入。编辑已有连接时，密码栏留空表示继续使用已存密码。移除已存密码后，需要保存配置才会生效。

保存已有连接的修改会断开该连接的旧会话。数据库列表由连接时或手动刷新时读取；保持一个会话不代表持续监测服务器可达性，刷新列表可以再次验证。

对象浏览只读取元数据，不读取表内容。单击数据库在中间对象列表显示该库的表，右侧保持库信息；单击表就会显示 DDL 标签，由用户手动切换后读取真实 `SHOW CREATE TABLE` DDL，双击表不会自动切换。复制表定义成功后，按钮文字后显示绿色 SVG 完成动画；失败则就地提示并允许重试。库节点的收起状态不影响对象列表，刷新或较晚完成的读取不会重新展开已收起的节点。表行数是服务器估计值；视图部分字段可能为空。每次最多列出 10,000 个对象，超出时会提示。查询、备份、账户、模型等未实现功能为禁用状态。

连接菜单按状态只显示“打开连接”或“关闭连接”。双击已打开的连接展开/收起。连接弹窗高度为 540px，可拖动标题栏且不限制偏移，双击标题栏或 Alt+Home 可重新居中。

## 数据位置

配置位于 `os.UserConfigDir()/BajiuDB/connections.json`：

- Windows：`%APPDATA%/BajiuDB/connections.json`
- macOS：`~/Library/Application Support/BajiuDB/connections.json`
- Linux：`$XDG_CONFIG_HOME/BajiuDB/connections.json`，未设置时通常为 `~/.config/BajiuDB/connections.json`

配置中只包含连接参数和凭据引用，不包含明文密码。凭据服务名为 `BajiuDB.connections`；Windows 使用 Credential Manager，macOS 使用 Keychain，Linux 使用 Secret Service。系统凭据库不可用时可以不记住密码，不会退回明文存储。配置文件可以备份，但密码不会随 JSON 文件跨机器迁移。

应用不提供云同步、遥测或自动更新功能。

## 开发与构建

当前锁定 Wails **v2.11.0**、Go 工具链 **1.26.11**，使用 Node.js **22.20.0**。Go 可按 go.mod 自动下载项目工具链；产品运行时不需要 Go。Go 模块与 npm 依赖均有锁文件。修改框架版本时应重新运行构建与集成测试。

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

产品版本保持 **0.0.1**，仅在项目所有者明确要求升级时修改。关于窗口直接读取前端包版本；构建前会检查 Wails、前端包及锁文件的版本一致性。

Windows 便携包运行 `./scripts/package.ps1`，固定输出到 `dist/BajiuDB-0.0.1-windows-x64/`、同名 `.zip` 和 `.sha256`。新包构建和哈希校验成功后覆盖同版本旧包，只保留最新一份；不同正式版本不自动清理。

若旧程序正从 `build/bin/BajiuDB.exe` 运行，打包会保留旧窗口，避免丢失未保存的表单；新程序仍会更新到 `dist`。关闭旧窗口后从发布目录打开新版。

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

可选集成测试使用调用者提供的独立 MySQL 测试实例，不读取本机已有应用的账户或密码。测试服务器需含 `bajiu_test_db` 数据库、`customers` 表及 `active_customers` 视图，使用不受信任的测试证书，并为测试账户授予该数据库的读取权限。以下 SQL 仅在专用测试库执行：

```sql
CREATE DATABASE IF NOT EXISTS bajiu_test_db;
CREATE TABLE IF NOT EXISTS bajiu_test_db.customers (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(80) NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE
) ENGINE=InnoDB COMMENT='客户资料';
CREATE OR REPLACE VIEW bajiu_test_db.active_customers AS
  SELECT id, name FROM bajiu_test_db.customers WHERE active=TRUE;
```

PowerShell 环境变量示例，以下凭据仅供测试：

```powershell
$env:BAJIU_MYSQL_TEST_ADDR = '127.0.0.1:33179'
$env:BAJIU_MYSQL_TEST_USER = 'bajiu_test'
$env:BAJIU_MYSQL_TEST_PASSWORD = 'bajiu-test-only-2026'
$env:BAJIU_TEST_KEYRING = '1'

# 可选：先按 tunnel/README.md 在专用测试网站部署脚本。
# $env:BAJIU_HTTP_TEST_URL = 'http://127.0.0.1:34119/bajiu-mysql.php'
# $env:BAJIU_HTTP_TEST_TOKEN = '<仅用于隔离测试的令牌>'
go test ./internal/... -count=1 -v
```

集成测试检查真实服务器登录、元数据读取、错误密码、TLS 验证失败，以及系统凭据库的存取和清理。MySQL 测试启用时也通过临时 SSH 服务器验证密码、加密私钥、主机指纹及表 DDL；配置 HTTP 测试变量后还验证真实 PHP 中转、令牌拒绝与 DDL。未设置变量时跳过相关测试。

## 代码结构

- `internal/connections`：配置、凭据、MySQL 访问、会话生命周期和测试。
- `app.go`：桌面方法桥接与本机文件选择。
- `main.go`：窗口和桌面生命周期。
- `frontend/src`：中文界面、交互和界面流程测试。
- `build`：图标、平台资源与本机编译产物。
- `THIRD_PARTY_NOTICES.md`、`licenses/`：第三方声明及许可证。

第三方组件按各自许可证分发；应用自身的发布许可由项目所有者决定。MySQL 驱动采用 MPL-2.0，对应源码和获取方式见第三方声明。
