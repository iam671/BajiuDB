# 数据库连接

版本仍为 0.0.1。点击“新建连接”先打开类型选择窗口，提供八种数据库、搜索和最近使用；点击类型后打开对应的连接设置，配置窗口不再提供类型下拉。支持测试、保存、打开、关闭和分组。旧连接配置自动识别为 MySQL；已保存连接的类型固定，需要其它类型时新建连接，避免已打开工作页被换到另一种数据库协议。

| 类型 | 默认端口 / 默认库 | 对象与查询 | 当前适配范围 |
| --- | --- | --- | --- |
| MySQL | 3306 / 可留空 | 原有数据库、表数据编辑、设计、事务、SQL、向导等 | 保留直连、SSH 和 PHP 隧道 |
| MariaDB | 3306 / 可留空 | 复用 MySQL 协议和现有管理界面 | 部分高级 DDL/排序规则取决于服务器版本 |
| PostgreSQL | 5432 / postgres | 可访问数据库、用户模式中的表/视图、原生 SQL 及结果 | 用户名/密码直连、可选验证 TLS |
| SQL Server | 1433 / master | 可访问数据库、表/视图、原生 T-SQL 及结果 | SQL 账户用户名/密码直连；填写实例实际 TCP 端口，可选验证 TLS |
| Oracle | 1521 / 必填 Service Name | 账户可见模式、表/视图、原生 SQL 及结果 | 用户名/密码与服务名，纯 Go 驱动，无需 Oracle 客户端；可选验证 TLS |
| SQLite | 本地已有文件 / main | 表/视图、原生 SQL 及结果 | 使用现有普通文件，读写连接；不自动创建不存在的路径，不支持 SQLCipher 密码库 |
| Redis | 6379 / 逻辑库 0 | SCAN 键浏览、类型、JSON 数组命令及结果 | 单端点直连，密码或 ACL 用户名/密码，可选验证 TLS；不包含 Sentinel/Cluster 路由配置 |
| MongoDB | 27017 / 可留空，认证库 admin | 可访问数据库、集合/视图、JSON 原生命令、Extended JSON 结果 | 单个种子主机、用户名/密码或服务端允许的匿名连接，可选验证 TLS；无 SRV URI/SSH 配置入口 |

新增类型的查询结果当前只读；执行的原生 SQL/命令本身可以按服务器权限修改数据。MySQL 专用的表设计、数据编辑、事务、SQL 转储和导入导出向导暂不用于其它引擎，相关菜单隐藏。MariaDB 保留 MySQL 兼容管理入口。每次 SQL/命令执行独立会话，跨次事务状态不保留。

## 连接设置

- 八种类型使用各自的原创 SVG 图标，类型选择、连接树、对象列表、信息、配置、查询与状态区保持一致。连接绿点表示已打开，灰点表示已关闭；引擎本身的颜色保留，数据库节点继续使用原有灰/绿状态。
- 工具栏、文件菜单、右键菜单和 Ctrl+N 统一先选择类型；编辑、打开和复制已有连接直接沿用原类型。最近使用只缓存版本化的类型 ID，不缓存密码、连接名称或服务器地址。
- 网络连接填写主机、端口、账户；Redis 可以省略用户名，MongoDB 可使用服务端允许的匿名认证。
- Oracle 在“数据库”页填写服务名，不是 SID；打开后左侧按可见模式浏览。
- MongoDB 在“数据库”页填写认证数据库，通常为 admin。应用不把密码放进 URI、配置 JSON 或日志。
- Redis 的“默认数据库”是非负编号。左侧显示所选逻辑库及 INFO keyspace 可见的非空逻辑库，不假设服务器一定配置 16 个库。需要其它空库时，可新建连接指定对应编号。
- SQLite 在常规页选择文件；网络主机、端口、账户、TLS 和隧道不适用。
- 新增网络引擎使用直连。MySQL/MariaDB 保留已有 SSH/PHP 配置；其它类型不显示这两个隧道页，也不会把其数据发送到 MySQL PHP 脚本。
- 勾选 TLS 后验证证书链和主机名，可选 PEM CA、证书主机名；不使用跳过证书验证选项。MongoDB 未指定自定义证书主机名时，对发现的各个节点按各自名称验证。
- 密码继续由系统凭据库保护；配置文件只存密码引用。“不保存密码”时凭据只在当前应用内存中使用。

## 查询示例

PostgreSQL / SQL Server / Oracle / SQLite 使用对应引擎的原生语法，双击表会生成正确引用名称、带预览数量限制的查询；视图可在原生 SQL 中访问。普通查询预览保留 1,000 行 / 8 MB 边界；这与 MySQL/MariaDB 向导的全量导出是独立功能。

Redis 输入 JSON 数组，二进制参数可使用 HEX 对象：

```json
["GET", "my-key"]
["HSCAN", "my-hash", "0", "COUNT", "1000"]
["GET", {"hex":"00ff"}]
```

Redis 每次命令使用独立连接，不保留 AUTH/SELECT/事务状态；查询窗口不运行持续订阅和 MONITOR。键名统一带引号并转义，区分空键、控制字符、二进制键和看似 HEX 标记的普通键；预览仍使用原始字节。扫描的键在同时被修改时可能变化，类型和内容以服务器响应为准。

MongoDB 输入 JSON 或 Extended JSON 命令，命令名放在首项：

```json
{"find":"users","filter":{},"limit":1000,"singleBatch":true}
{"ping":1}
```

响应以 Extended JSON 显示，保留 ObjectId、Int64 等类型。原生命令返回服务器当前响应；需要后续游标批次时使用返回的游标及 getMore，不伪装为已经读取全部数据。

## 验证与驱动

使用实际 Go 驱动：pgx、Microsoft go-mssqldb、go-ora、go-redis、MongoDB Go Driver；SQLite 使用已有 modernc SQLite。驱动及其依赖许可随发布包附带。参考 [SQL Server 驱动配置](https://learn.microsoft.com/en-us/sql/connect/golang/connection-strings)、[Oracle 驱动](https://github.com/sijms/go-ora)、[MongoDB 驱动配置](https://www.mongodb.com/docs/drivers/go/current/connect/specify-connection-options/)。

本次实测 SQLite 临时数据库和独立启动、自动关闭的 Redis 测试实例；其它服务器类型已接入驱动、通过编译和配置/界面检查，尚未进行真实服务器端到端验证。不会为验证连接而访问已有业务数据库。具体检查记录见 VALIDATION.md。
