<div align="center">

# Bajiu DB · 捌玖

**轻量级数据库管理工具，让日常数据库操作更简单。**

连接管理、SQL 查询、表结构设计与数据编辑，一个桌面工作区完成。<br/>
基于 Go + Wails + Vue，使用系统 WebView。

[English](README.md) · **简体中文**

[GitHub](https://github.com/iam671/BajiuDB) · [Gitee](https://gitee.com/sixeventy-one/bajiu-db) · [使用文档](docs/README.zh-CN.md) · [问题反馈](https://github.com/iam671/BajiuDB/issues) · [支持开发](#支持开发)

![版本](https://img.shields.io/badge/version-0.0.1-1685d1)
[![许可证](https://img.shields.io/badge/license-Apache--2.0-2b8a57)](LICENSE)
![技术栈](https://img.shields.io/badge/Go-Wails-Vue-475569)

</div>

## 为什么选择 Bajiu DB？

Bajiu DB（捌玖）是一款开源桌面数据库管理工具，专注连接、查询、结构查看和数据操作。Go 后端配合系统 WebView，应用无需再打包一套独立浏览器内核。

- **轻量桌面应用**：当前 Windows x64 ZIP 约 19 MB、EXE 约 57 MB（十进制），WebView2 运行时单独安装。体积随构建变化，暂无内存或速度跑分承诺。
- **八种数据库，一个入口**：统一管理关系型数据库、Redis 与 MongoDB 连接，各引擎使用专属图标及连接状态标识。
- **清晰的工作布局**：左侧连接分组，中间可关闭工作标签，右侧对象信息与 DDL，减少来回切换。
- **配置保存在本机**：密码交给系统凭据库，不提供内置云同步、遥测或自动更新。

当前版本为 **0.0.1**，应用界面为简体中文，文档提供中英文版本。当前提供 Windows x64 打包流程；macOS/Linux 桌面构建及运行效果仍待验证。

## 功能特性

### 连接与对象浏览

新建连接先选择数据库类型，再填写对应设置。支持测试、保存、复制、打开和关闭连接，连接分组与拖拽归组；按各引擎协议浏览数据库、模式和对象。MySQL/MariaDB 还提供验证证书的 TLS、SSH 隧道及配套网站 URL/PHP 中转。

### 查询与数据编辑

- 查询以可关闭 Tab 打开，分别保留 SQL、结果及数据库上下文。
- MySQL/MariaDB 表数据支持网格/表单、筛选排序、列显示与列宽记忆，虚拟滚动，每页 1,000 条。
- 双击单元格编辑，修改先留在草稿，点击“保存”才写入数据库；支持复制原值、Insert/Update 语句及制表符文本。
- 日期时间、ENUM/SET、精确数值、JSON/HEX 等类型提供快捷编辑。
- 符合条件的单表 SELECT 结果共用表编辑器；修改已有记录需有可用的非空唯一键，不能可靠确定来源的结果只读。
- 直连/SSH 的适用 InnoDB 表支持独立事务；查询批次与命令行每次提交使用独立会话。

### 结构与数据工具

MySQL/MariaDB 支持新建/设计表，编辑字段、索引、外键、触发器、注释与选项，查看 SQL 预览；管理数据库、运行 SQL 文件、转储结构和数据或仅结构、生成数据字典，以及按真实键关系生成 SVG 模型。

导入/导出向导支持 **TXT、CSV、JSON、XML、XLS、XLSX、MDB、ACCDB、ODBC、DBF、SQLite DB**。导出在后端流式处理，不设应用层总行数和总文件大小上限；导入另有预览及大小限制，部分格式依赖 Windows 驱动。详见[格式说明](docs/data-transfer.zh-CN.md)。

## 支持的数据库

| 数据库 | 当前能力 |
| --- | --- |
| MySQL | 连接、浏览、查询、数据编辑、表设计、事务、SQL 转储及数据向导 |
| MariaDB | MySQL 兼容管理流程，高级语法以服务器版本为准 |
| PostgreSQL | 连接、数据库/模式与表/视图浏览、原生 SQL |
| SQL Server | SQL 账户认证、数据库/对象浏览、原生 T-SQL |
| Oracle | Service Name 连接、可见模式/对象浏览、原生 SQL |
| SQLite | 打开已有数据库文件、对象浏览、执行 SQL |
| Redis | 逻辑库、键扫描与类型查看、JSON 数组命令 |
| MongoDB | 认证库、集合浏览、JSON/Extended JSON 命令 |

PostgreSQL、SQL Server、Oracle、SQLite、Redis、MongoDB 的查询结果当前只读，原生 SQL/命令仍可按服务端权限写入。它们暂不提供 MySQL 专用表设计、数据向导或事务工具。连接选项与实际验证范围见[数据库指南](docs/database-engines.zh-CN.md)。

## 下载与快速开始

- [GitHub 发布页](https://github.com/iam671/BajiuDB/releases)
- [Gitee 发布页](https://gitee.com/sixeventy-one/bajiu-db/releases)

若发布页尚未附带安装包，可以[从源码构建](docs/development.zh-CN.md)。Windows 便携包名称为 `BajiuDB-0.0.1-windows-x64.zip`。

1. 解压便携包，运行 `BajiuDB.exe`；缺少 WebView2 时先安装 Microsoft WebView2 Runtime。
2. 点击“连接”，选择数据库类型，再填写连接参数。
3. 测试并保存连接，打开后开始浏览；SQLite 需选择已存在的数据库文件。
4. 双击数据库或对象打开相应页面。MySQL/MariaDB 表进入数据标签，其它引擎使用原生查询预览。

详细操作见[安装配置](docs/installation.zh-CN.md)及[使用指南](docs/user-guide.zh-CN.md)。

## 从源码构建

使用 Go **1.26.11**、Node.js **22.20.0**，并安装当前系统所需的 Wails **3.1.0** 原生依赖。

```sh
git clone https://github.com/iam671/BajiuDB.git
# 也可使用：git clone https://gitee.com/sixeventy-one/bajiu-db.git
cd BajiuDB
npm ci --prefix frontend
npm run build --prefix frontend
go run .
```

使用 Gitee 克隆时目录名为 `bajiu-db`，请进入对应目录。Windows 执行 `./scripts/build.ps1` 生成 EXE，执行 `./scripts/package.ps1` 生成便携包、校验文件和随包文档；无需安装 Wails CLI。开发检查与平台说明见[开发与贡献](docs/development.zh-CN.md)。

## 文档

项目指南全部使用仓库内 Markdown 文件维护，无需外部文档空间。

| 内容 | 简体中文 | English |
| --- | --- | --- |
| 文档目录 | [阅读](docs/README.zh-CN.md) | [Read](docs/README.md) |
| 安装与配置 | [阅读](docs/installation.zh-CN.md) | [Read](docs/installation.md) |
| 日常使用 | [阅读](docs/user-guide.zh-CN.md) | [Read](docs/user-guide.md) |
| 数据库与能力范围 | [阅读](docs/database-engines.zh-CN.md) | [Read](docs/database-engines.md) |
| 导入导出格式 | [阅读](docs/data-transfer.zh-CN.md) | [Read](docs/data-transfer.md) |
| SSH / PHP 隧道 | [阅读](tunnel/README.zh-CN.md) | [Read](tunnel/README.md) |
| 开发与贡献 | [阅读](docs/development.zh-CN.md) | [Read](docs/development.md) |

## 参与贡献与反馈

欢迎通过 [GitHub](https://github.com/iam671/BajiuDB/issues) 或 [Gitee](https://gitee.com/sixeventy-one/bajiu-db/issues) 提交问题、复现示例、文档改进和聚焦的 Pull Request。反馈时请提供系统、应用及数据库版本、复现步骤；截图、日志和 SQL 示例请移除密码及私人数据。每个问题单独反馈，PR 面向 `main`。

## 支持开发

如果 Bajiu DB 对你有帮助，欢迎 Star、反馈问题或自愿赞助。请使用对应支付应用扫码，点击图片可以查看原图。

| 支付宝 | 微信支付 |
| :---: | :---: |
| <a href="docs/assets/alipay.jpg"><img src="docs/assets/alipay.jpg" alt="支付宝赞助收款码" width="260"/></a> | <a href="docs/assets/wechat-pay.jpg"><img src="docs/assets/wechat-pay.jpg" alt="微信支付赞助收款码" width="260"/></a> |

## 开源许可证

Bajiu DB 使用 [Apache License 2.0](LICENSE)。第三方组件保留各自许可，详见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)、[licenses](licenses/) 及随仓库附带的 [MySQL 驱动源码](third_party_sources/)。收款码由作者提供，用于自愿赞助入口。
