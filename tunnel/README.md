# Website URL / PHP tunnel and SSH

**English** · [简体中文](README.zh-CN.md) · [Documentation](../docs/README.md)

The bundled scripts implement Bajiu DB's own `bajiu-mysql-v1` protocol for MySQL/MariaDB. They are not interchangeable with another application's tunnel scripts. PHP 8.1+, mysqli, mysqlnd and a valid HTTPS site are required.

## Deploy the PHP tunnel

1. Upload `bajiu-mysql.php` and `admin.php` into the same site directory, for example `https://db.example.com/bajiu-mysql.php`. Replace both files when updating.
2. Set `BAJIU_TUNNEL_TOKEN` in the PHP process environment to a random value of at least 32 characters. Generate one with `php -r "echo bin2hex(random_bytes(32));"`. Keep it in the server environment and client credential store only.
3. Set `BAJIU_TUNNEL_ALLOWED_HOSTS` to a comma-separated allowlist of reachable database hosts. The default is `127.0.0.1,localhost,::1`; explicitly add any remote MySQL hostname/IP.
4. In the connection's general settings, enter the MySQL address/port/account as seen from the PHP server. Enable the HTTP tunnel, supply the script's HTTPS URL and matching token, then test.
5. For query/file execution and database creation/editing/deletion, explicitly set `BAJIU_TUNNEL_ALLOW_SQL=1` in the PHP environment. Without it, metadata, dictionary/model and dump remain available but SQL execution is disabled. Enabling it lets a holder of the token and database credentials execute SQL within that account's permissions.

Example PHP-FPM pool configuration; replace the placeholder before reloading the service:

```ini
env[BAJIU_TUNNEL_TOKEN] = replace-with-your-own-random-token-at-least-32-characters
env[BAJIU_TUNNEL_ALLOWED_HOSTS] = 127.0.0.1,localhost
env[BAJIU_TUNNEL_ALLOW_SQL] = 1
```

The gateway authenticates with `X-Bajiu-Token`, independently of website Basic authentication. URLs cannot embed credentials, query parameters or redirects. HTTPS uses the system trust store and cannot skip certificate verification. If database SSL is enabled, CA content is sent over HTTPS, used temporarily on the server and deleted; the database hostname must match the certificate name.

## Scope and sessions

The tunnel supports connection tests, database lists, table/view/routine/event metadata, table DDL, dictionaries, models and dumps. SQL execution and management require the explicit SQL switch. Each request opens/closes its own database connection: an open client profile means the tunnel settings were validated, not that the website keeps a persistent connection or cross-request transaction. Put a complete transaction in one batch.

PHP requests use the connection timeout with a maximum of 30 seconds. Stopping the client wait does not guarantee stopping server SQL. SQL dumps have an approximately 48 MB bound and omit triggers, routines and events. Full table export uses a separate streaming/paged route described in the [format guide](../docs/data-transfer.md); it does not provide a cross-request snapshot.

The endpoint only accepts POST, bounds request/response sizes and does not return database passwords or raw internal exceptions. MySQL still enforces database privileges. Plain HTTP is only suitable for controlled test networks; use HTTPS for normal deployment.

Authenticated SQL failures return the MySQL error number and server message for syntax diagnostics. Details are displayed in the client, not logged by the application. Keep `admin.php` up to date to receive complete messages and the structured `errorCode`, including duplicate-table errors.

## Local testing

The PHP built-in server below is for isolated development only:

```powershell
$env:BAJIU_TUNNEL_TOKEN = 'replace-with-your-own-random-token-at-least-32-characters'
php -S 127.0.0.1:34119 -t tunnel
```

Database TLS uses mysqli settings and validates the negotiated SSL cipher; encrypted mode refuses a connection without negotiated encryption.

## SSH

SSH supports a password or OpenSSH/PEM private key with an optional key passphrase. Verify the host using the local `.ssh/known_hosts`, a selected known_hosts file or a SHA256 fingerprint checked with the administrator in advance. Unknown host keys are not automatically accepted.

The general-page database address is resolved from the SSH server. If the database is on the same host, use `127.0.0.1:3306` there and the public SSH host/account on the SSH page. The SSH server must allow `direct-tcpip`. SSH and PHP tunnel modes are mutually exclusive.
