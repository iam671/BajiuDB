# 开发与贡献

[English](development.md) · **简体中文** · [文档目录](README.zh-CN.md)

## 工具链

| 组件 | 仓库版本 |
| --- | --- |
| Go | `go.mod` 声明 1.26.0，选择工具链 1.26.11 |
| Node.js | 开发基线 22.20.0 |
| Wails | Go 包 3.1.0，JavaScript runtime 3.1.0 |
| 前端 | Vue 3、TypeScript、Vite，精确依赖版本见 `frontend/package-lock.json` |

允许联网时 Go 可自动下载指定工具链。需安装目标系统对应的 Wails 原生依赖：Windows 使用 WebView2，macOS 使用系统 WebKit，Linux 需要相应开发库。当前已检查的打包流程为 Windows x64，macOS/Linux 桌面构建和运行仍待验证。

## 克隆与运行

```sh
git clone https://github.com/iam671/BajiuDB.git
cd BajiuDB
npm ci --prefix frontend
npm run build --prefix frontend
go run .
```

也可克隆 `https://gitee.com/sixeventy-one/bajiu-db.git`，然后进入 `bajiu-db`。两端均使用 `main`。

仅开发前端可运行 `npm run dev --prefix frontend`，但浏览器预览无法验证原生多窗口、系统凭据及 Go 桥接。Go 应用嵌入 `frontend/dist`，首次运行前须先构建前端；修改后也需重新构建，才会更新嵌入资源。

## 构建与打包

Windows PowerShell 中，在仓库根目录执行：

```powershell
./scripts/build.ps1
./scripts/package.ps1
```

`build.ps1` 安装/构建前端，生成 Windows 图标、版本和 manifest，再编译生产 EXE，无需全局安装 Wails CLI。`package.ps1` 加入 Markdown 指南、本地图片、隧道脚本、项目许可证、第三方许可及 MySQL 驱动源码，并检查包内 EXE 哈希。

**0.0.1** 的输出名称：

```text
build/bin/BajiuDB.exe
dist/BajiuDB-0.0.1-windows-x64/
dist/BajiuDB-0.0.1-windows-x64.zip
dist/BajiuDB-0.0.1-windows-x64.sha256
```

打包中间产物可使用 `BajiuDB-package.exe`。成功构建后覆盖同版本固定发布包，不累积日期副本；正在运行的旧程序会保留，需关闭后再打开新版。连接配置不属于构建产物，不得随清理删除。

维护者明确要求升级前，产品版本保持 0.0.1；`wails.json`、前端包元数据及锁文件必须一致。

## 检查

```sh
go test ./...
go vet ./...
npx --prefix frontend vitest run --root frontend --exclude src/App.test.ts
npm run build --prefix frontend
```

`frontend/src/App.test.ts` 的历史失败已在 [VALIDATION.md](../VALIDATION.md) 记录，上述聚焦检查排除了它。`npm test --prefix frontend` 会执行包含该文件的完整套件，不能把排除项报告为通过。

根 Go 包检查需放在前端构建之后，不能与 Vite 同时运行，因为构建时会暂时清空嵌入资源目录。前端检查也不能与打包同时进行，`npm ci` 会重建 `node_modules`。纯文档修改检查链接及资源即可，不应声称已重测运行行为。

### 可选集成检查

使用独立可丢弃 MySQL 实例，准备 `bajiu_test_db` 数据库、`customers` 表（`id` BIGINT 主键、`name` VARCHAR、`active` BOOLEAN）及 `active_customers` 视图。测试账户需有读取权限，TLS 失败检查使用不受信任测试证书。不得使用已有业务库或个人保存的凭据。

```powershell
$env:BAJIU_MYSQL_TEST_ADDR = '127.0.0.1:33179'
$env:BAJIU_MYSQL_TEST_USER = 'bajiu_test'
$env:BAJIU_MYSQL_TEST_PASSWORD = 'replace-with-isolated-test-password'
$env:BAJIU_TEST_KEYRING = '1'
go test ./internal/... -count=1 -v
```

`BAJIU_TEST_KEYRING=1` 会启用真实系统凭据库操作；启用 MySQL 的检查还会使用临时 SSH 服务。PHP 可选检查需在独立测试站点部署两个脚本，设置 `BAJIU_HTTP_TEST_URL` 和 `BAJIU_HTTP_TEST_TOKEN`，见[隧道部署](../tunnel/README.zh-CN.md)。缺少相应变量时跳过对应集成检查；文件驱动检查及平台条件见 [VALIDATION.md](../VALIDATION.md)。

## 代码结构

| 路径 | 职责 |
| --- | --- |
| `internal/connections/` | 引擎适配、配置、凭据、会话及数据库操作 |
| `internal/datafiles/` | 文件格式及流式传输 |
| `app.go`、`*_app.go` | 暴露给桌面客户端的 Go 方法 |
| `main.go`、`native_dialogs.go` | 应用与原生窗口生命周期 |
| `frontend/src/` | Vue 界面、交互逻辑及检查 |
| `scripts/`、`tools/resourcegen/` | 构建、打包、许可收集及 Windows 资源 |
| `docs/`、`tunnel/` | Markdown 指南与 PHP 中转部署 |
| `licenses/`、`third_party_sources/` | 第三方分发材料 |

## 参与贡献

先阅读 [AGENTS.md](../AGENTS.md) 和[注意事项](pitfalls.md)。在 [GitHub](https://github.com/iam671/BajiuDB) 或 [Gitee](https://gitee.com/sixeventy-one/bajiu-db) 提交问题或面向 `main` 的聚焦 PR，说明问题、最终行为、检查结果及未验证平台/数据库。功能变化时同步更新两种语言的指南。

不要提交凭据、连接配置、私人数据或构建产物。布局修改需保留草稿/保存语义，新增引擎需明确能力范围，不能误发其它引擎的 SQL。项目代码使用 [Apache-2.0](../LICENSE)，依赖按[第三方声明](../THIRD_PARTY_NOTICES.md)保留各自许可。
