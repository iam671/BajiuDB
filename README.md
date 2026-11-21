<div align="center">

# Bajiu DB · 捌玖

**A lightweight database manager for your desktop.**

Connections, queries, table design and data editing in one workspace.<br/>
Built with Go + Wails + Vue and the system WebView.

**English** · [简体中文](README.zh-CN.md)

[GitHub](https://github.com/iam671/BajiuDB) · [Gitee](https://gitee.com/sixeventy-one/bajiu-db) · [Documentation](docs/README.md) · [Report an issue](https://github.com/iam671/BajiuDB/issues) · [Support development](#support-development)

![Version](https://img.shields.io/badge/version-0.0.1-1685d1)
[![License](https://img.shields.io/badge/license-Apache--2.0-2b8a57)](LICENSE)
![Stack](https://img.shields.io/badge/Go-Wails-Vue-475569)

</div>

## Why Bajiu DB?

Bajiu DB is an open-source desktop database manager focused on everyday work: organize connections, inspect schemas, run queries and edit data without a heavy desktop shell. It uses a Go backend and the operating system's WebView, so the application does not bundle a separate browser engine.

- **A compact desktop app.** The current Windows x64 ZIP is about 19 MB (decimal); the executable is about 57 MB. WebView2 is a separate runtime. Sizes vary with each build; no memory or speed benchmark is claimed.
- **Eight database engines.** Keep SQL databases, Redis and MongoDB connections in one tree, with recognizable engine icons and connection status.
- **A familiar workspace.** Connection groups on the left, closable working tabs in the center, and object information and DDL on the right.
- **Local configuration.** Save passwords in the system credential store. The application has no built-in cloud synchronization, telemetry or automatic updates.

Current version: **0.0.1**. The interface is currently in Simplified Chinese. Documentation is available in English and Chinese. Windows x64 is the currently packaged platform; macOS/Linux desktop builds and runtime behavior still need validation.

## Features

### Connections and browsing

Choose a database type before entering connection settings. Test, save, duplicate, open and close connections; organize them in groups and drag connections between groups. Browse available databases, schemas and objects with engine-specific queries. MySQL/MariaDB also support verified TLS, SSH tunnels and the bundled website URL/PHP tunnel.

### Queries and data editing

- Closable query tabs preserve their own SQL, results and database context.
- MySQL/MariaDB table tabs support grid/form views, filtering, sorting, column visibility, remembered column widths and virtualized rows, with 1,000 records per page.
- Double-click a cell to edit it. Changes stay in a draft until **Save**. Copy values or generate INSERT/UPDATE and tab-separated text.
- Date/time pickers, ENUM/SET choices, precise numeric operations and JSON/HEX editors help enter typed values.
- Eligible direct single-table SELECT results can reuse the table editor. Existing-row updates require a usable non-null unique key; ambiguous query results remain read-only.
- Dedicated table transactions are available for eligible InnoDB tables over direct/SSH connections. Query batches and the SQL console do not maintain a session between submissions.

### Schema and data tools

For MySQL/MariaDB: create and edit tables, fields, indexes, foreign keys, triggers, comments and options; inspect generated SQL; manage databases; run SQL files; dump structure with or without data; generate data dictionaries and SVG models from declared keys.

Import/export wizards support **TXT, CSV, JSON, XML, XLS, XLSX, MDB, ACCDB, ODBC, DBF and SQLite DB**. Export streams records in the backend without an application-wide total row/file-size cap. Import has separate preview and size limits; some formats need Windows drivers. See [format requirements](docs/data-transfer.md).

## Supported databases

| Database | Current capabilities |
| --- | --- |
| MySQL | Connection management, browsing, queries, data editing, table design, transactions, SQL dump and transfer tools |
| MariaDB | MySQL-compatible management workflow; advanced syntax depends on the server version |
| PostgreSQL | Connections, database/schema browsing, tables/views and native SQL |
| SQL Server | SQL authentication, database/object browsing and native T-SQL |
| Oracle | Service Name connections, visible schemas/objects and native SQL |
| SQLite | Open an existing database file, browse objects and run SQL |
| Redis | Logical databases, key scanning/type inspection and JSON-array commands |
| MongoDB | Authentication databases, collection browsing and JSON/Extended JSON commands |

PostgreSQL, SQL Server, Oracle, SQLite, Redis and MongoDB currently display query results read-only. Native SQL/commands may still write according to server permissions. They do not yet expose the MySQL-specific table designer, transfer or transaction tools. See the [engine guide](docs/database-engines.md) for connection options and validation status.

## Download and get started

- [GitHub releases](https://github.com/iam671/BajiuDB/releases)
- [Gitee releases](https://gitee.com/sixeventy-one/bajiu-db/releases)

If no binary is attached to a release, [build from source](docs/development.md). The expected Windows archive name is `BajiuDB-0.0.1-windows-x64.zip`.

1. Extract the Windows package and launch `BajiuDB.exe`. Install Microsoft WebView2 Runtime if it is missing.
2. Select **连接 (Connection)**, choose a database engine, then enter its connection settings.
3. Test and save the connection, then open it. SQLite requires an existing database file.
4. Double-click a database/object to browse it. MySQL/MariaDB tables open in editable tabs; other engines use native query previews.

Read the [installation guide](docs/installation.md) and [user guide](docs/user-guide.md) for details.

## Build from source

Use Go **1.26.11**, Node.js **22.20.0** and the native dependencies for Wails **3.1.0** on your OS.

```sh
git clone https://github.com/iam671/BajiuDB.git
# Alternative: git clone https://gitee.com/sixeventy-one/bajiu-db.git
cd BajiuDB
npm ci --prefix frontend
npm run build --prefix frontend
go run .
```

For a Gitee clone, enter the `bajiu-db` directory instead. On Windows, `./scripts/build.ps1` builds the EXE and `./scripts/package.ps1` creates the portable release, checksums and bundled documentation. These scripts do not require the Wails CLI. See [development and contribution](docs/development.md) for checks and platform notes.

## Documentation

All project guides are versioned Markdown files in this repository. No external document workspace is required.

| Topic | English | 简体中文 |
| --- | --- | --- |
| Documentation index | [Read](docs/README.md) | [阅读](docs/README.zh-CN.md) |
| Installation and configuration | [Read](docs/installation.md) | [阅读](docs/installation.zh-CN.md) |
| Everyday usage | [Read](docs/user-guide.md) | [阅读](docs/user-guide.zh-CN.md) |
| Database engines and current scope | [Read](docs/database-engines.md) | [阅读](docs/database-engines.zh-CN.md) |
| Import/export formats | [Read](docs/data-transfer.md) | [阅读](docs/data-transfer.zh-CN.md) |
| SSH and PHP tunnels | [Read](tunnel/README.md) | [阅读](tunnel/README.zh-CN.md) |
| Development and contribution | [Read](docs/development.md) | [阅读](docs/development.zh-CN.md) |

## Contribute and give feedback

Bug reports, reproducible examples, documentation improvements and focused pull requests are welcome on [GitHub](https://github.com/iam671/BajiuDB/issues) or [Gitee](https://gitee.com/sixeventy-one/bajiu-db/issues). Include your OS, application version, engine/server version and reproduction steps. Remove credentials and private data from screenshots, logs and SQL examples. Please keep one issue per report and one focused change per pull request, targeting `main`.

## Support development

If Bajiu DB helps your work, a star, useful bug report or optional donation supports continued development. Scan with the matching payment app; click an image to open the original.

| Alipay | WeChat Pay |
| :---: | :---: |
| <a href="docs/assets/alipay.jpg"><img src="docs/assets/alipay.jpg" alt="Alipay donation QR code" width="260"/></a> | <a href="docs/assets/wechat-pay.jpg"><img src="docs/assets/wechat-pay.jpg" alt="WeChat Pay donation QR code" width="260"/></a> |

## License

Bajiu DB is licensed under [Apache License 2.0](LICENSE). Third-party components retain their own licenses; see [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md), [licenses](licenses/) and the bundled [MySQL driver source](third_party_sources/). Payment QR images are provided by the maintainer for optional donations.
