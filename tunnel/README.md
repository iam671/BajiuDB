# 网站 URL / PHP 中转

这是 Bajiu DB 自有协议 `bajiu-mysql-v1` 的配套脚本，不兼容其他软件的中转脚本。使用 PHP 8.1+、mysqli、mysqlnd 和有效 HTTPS 网站。

1. 将 `bajiu-mysql.php` 和 `admin.php` 一起上传到网站同一目录，例如 `https://db.example.com/bajiu-mysql.php`。升级已有部署时两个文件一起替换。
2. 在 PHP 进程环境中设置 `BAJIU_TUNNEL_TOKEN`，至少 32 字符。可用 `php -r "echo bin2hex(random_bytes(32));"` 生成。令牌只放在服务端环境及客户端凭据库中。
3. 设置 `BAJIU_TUNNEL_ALLOWED_HOSTS` 为中转可访问的数据库主机，以逗号分隔。默认只允许 `127.0.0.1,localhost,::1`；需要远程 MySQL 时显式加入其主机名或 IP。
4. Bajiu DB 新建连接的常规页填写 **PHP 服务器视角**的 MySQL 主机、端口和账户。在 HTTP 页启用中转，填写脚本 HTTPS 地址及相同令牌，再测试连接。
5. 若需要查询、运行 SQL 文件和新建/编辑/删除数据库，在 PHP 进程环境显式设置 `BAJIU_TUNNEL_ALLOW_SQL=1`。未设置时仍可浏览元数据、查看字典/模型及转储，但不开放 SQL 执行。启用后，持有令牌和数据库账户的人可执行该账户权限内的 SQL。

例如 PHP-FPM 的 pool 配置（改成实际令牌后重载服务）：

```ini
env[BAJIU_TUNNEL_TOKEN] = replace-with-your-own-random-token-at-least-32-characters
env[BAJIU_TUNNEL_ALLOWED_HOSTS] = 127.0.0.1,localhost
env[BAJIU_TUNNEL_ALLOW_SQL] = 1
```

网关使用 `X-Bajiu-Token` 请求头认证，不依赖网站的 Basic 登录。URL 不允许内嵌用户名、密码、查询参数或重定向。HTTPS 证书按系统信任库验证，不提供跳过校验的选项。若启用数据库 SSL，CA 文件内容会随 HTTPS 请求发送，由 PHP 临时使用后删除；证书主机名须与数据库主机相同。

当前支持连接测试、数据库列表、表/视图/例程/事件元数据、表 DDL、字典、模型和转储；开启 SQL 开关后还可执行 SQL 和管理数据库。每次请求独立建立并关闭数据库连接，客户端“已打开”表示中转配置已验证并可继续请求，**不表示网站保持长连接或跨请求事务**。事务须放在同一批次提交。PHP 请求最长按连接设置等待 30 秒；取消客户端等待不保证取消服务端 SQL。转储最多约 48 MB，不包含触发器、例程和事件。

接口仅允许 POST、限制请求和响应大小、不返回数据库密码或原始异常。数据库账户仍按 MySQL 自身权限授权。HTTP 明文地址仅适合受控测试网络；正式使用应配置 HTTPS，避免传输账户、密码与令牌的明文。

本机开发检查示例（PHP 内置服务器只用于测试）：

```powershell
$env:BAJIU_TUNNEL_TOKEN = 'replace-with-your-own-random-token-at-least-32-characters'
php -S 127.0.0.1:34119 -t tunnel
```

PHP TLS 参数依据官方 [mysqli::ssl_set](https://www.php.net/manual/en/mysqli.ssl-set.php) 与 [mysqli::options](https://www.php.net/manual/en/mysqli.options.php)；脚本还检查已协商的 SSL cipher，拒绝以未加密数据库连接完成验证。

## SSH

SSH 使用密码或 OpenSSH/PEM 私钥，可填写加密私钥的口令。主机信任使用本机 `.ssh/known_hosts`、指定 known_hosts 文件，或事先向管理员核对的 SHA256 指纹。没有自动接受未知主机密钥的路径。

常规页的 MySQL 地址从 SSH 服务器出发访问。例如 MySQL 与 SSH 位于同一机器，常规页可填 `127.0.0.1:3306`，SSH 页填公网 SSH 主机与 SSH 账户。SSH 服务须允许 `direct-tcpip` 转发。SSH 与 PHP 中转二选一。
