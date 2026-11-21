# Database connections

**English** · [简体中文](database-engines.zh-CN.md) · [Documentation](README.md)

Version **0.0.1**. New connections start with an engine picker, search and recent types. Choosing an engine opens fixed-type settings; editing a saved connection cannot change its type. Older profiles without a type load as MySQL. Connections support testing, saving, opening, closing and grouping.

| Engine | Default port / database | Objects and queries | Current connection scope |
| --- | --- | --- | --- |
| MySQL | 3306 / optional database | Database browsing, table editing/design, transactions, SQL and transfer tools | Direct, SSH and bundled PHP tunnel |
| MariaDB | 3306 / optional database | MySQL-compatible management workflow | Advanced DDL/collations depend on the server version |
| PostgreSQL | 5432 / postgres | Accessible databases, user schemas, tables/views and native SQL | Username/password; optional verified TLS |
| SQL Server | 1433 / master | Accessible databases, tables/views and native T-SQL | SQL authentication; actual instance TCP port; optional verified TLS |
| Oracle | 1521 / required Service Name | Visible schemas/objects and native SQL | Username/password and service name; pure Go driver; optional verified TLS |
| SQLite | Existing file / main | Tables/views and SQL | Existing regular file, read/write connection; no automatic file creation or SQLCipher support |
| Redis | 6379 / logical database 0 | SCAN, key types and JSON-array commands | Single endpoint, password/ACL username; optional verified TLS; no Sentinel/Cluster routing settings |
| MongoDB | 27017 / optional database; auth database admin | Accessible databases, collections/views, JSON commands and Extended JSON results | Single seed host, username/password or server-permitted anonymous access; optional verified TLS; no SRV URI/SSH settings |

The six engines other than MySQL/MariaDB currently show read-only query results. Native SQL/commands can still write according to server permissions. Their UI hides MySQL-specific editing, design, transaction, dump and transfer tools. Query/command execution uses independent sessions; transaction state does not carry between submissions.

## Settings

- Original SVG engine icons are shared by the picker, connection tree/list, information, settings, queries and status area. Green/gray dots indicate open/closed connections while engine colors stay intact. Database nodes retain their separate gray/green state.
- Toolbar/menu/context-menu/Ctrl+N entry points all choose a type first. Editing, opening or duplicating an existing profile retains its type. Recent-type storage contains only versioned known engine IDs, not connection names, addresses or credentials.
- Network engines use host, port and account fields. Redis may omit the username. MongoDB may use anonymous access only when the server allows it.
- Oracle uses a Service Name, not a SID; browsing lists schemas visible to the account.
- MongoDB has a separate authentication database, normally `admin`. Passwords are not put in a URI, configuration JSON or logs.
- Redis's default database is a nonnegative integer. The tree includes the chosen database and nonempty databases reported by INFO keyspace; it does not assume every server has 16 databases. Create another connection to select a different empty logical database.
- SQLite selects an existing file. Network address, account, TLS and tunnel options do not apply.
- New network engines use direct connections. SSH/PHP pages remain specific to MySQL/MariaDB; other engines never send credentials or data to the MySQL PHP tunnel.
- TLS verifies certificate chains and hostnames. Optional PEM CA and certificate hostname settings are supported; there is no skip-verification switch. MongoDB validates each discovered node by its own name unless a custom certificate hostname is specified.
- Passwords use the system credential store. The JSON file stores references; an unremembered password stays only in process memory.

## Native query examples

PostgreSQL/SQL Server/Oracle/SQLite use native SQL. Opening an object generates a safely quoted preview query with a record limit. Ordinary query previews retain a 1,000-row / 8 MB boundary; MySQL/MariaDB full streaming exports use a separate path.

Redis accepts a JSON array. Binary parameters may use an explicit HEX object:

```json
["GET", "my-key"]
["HSCAN", "my-hash", "0", "COUNT", "1000"]
["GET", {"hex":"00ff"}]
```

Each Redis command has its own connection. AUTH/SELECT/transaction state does not persist; continuous subscriptions and MONITOR are not run in the query window. Key names are quoted and escaped to distinguish empty/control/binary keys and literal text resembling a HEX marker; previews use the original bytes. Concurrent writes may change scanned keys or types.

MongoDB accepts JSON or Extended JSON commands. Put the command name first:

```json
{"find":"users","filter":{},"limit":1000,"singleBatch":true}
{"ping":1}
```

Responses use Extended JSON to retain ObjectId, Int64 and other types. The server's current response is shown; use its cursor and `getMore` when more batches are required. A returned batch is not presented as the complete collection.

## Drivers and verification

Adapters use pgx, Microsoft go-mssqldb, go-ora, go-redis, MongoDB Go Driver and modernc SQLite, alongside the MySQL driver. Dependencies and license notices are included in distribution materials.

SQLite was exercised against temporary databases and Redis against a separately started disposable test server. Other network engines have driver/configuration/build/UI checks but still need live-server end-to-end validation. Existing business databases are not accessed for verification. See [VALIDATION.md](../VALIDATION.md) for the detailed record.
