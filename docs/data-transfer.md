# Data import and export

**English** · [简体中文](data-transfer.zh-CN.md) · [Documentation](README.md)

Version **0.0.1**. These wizards currently apply to MySQL/MariaDB tables.

| Format | Read/write behavior | Requirement |
| --- | --- | --- |
| TXT | UTF-8 delimited text with a header; tab, comma, semicolon or pipe | Built in |
| CSV | UTF-8 CSV, BOM, quoted/multiline values | Built in |
| JSON | Import object arrays or columns/rows; export columns/rows | Built in; numeric lexemes preserved |
| XML | Import this app's XML or simple fields under row/record; export data/columns/rows | Built in; text fields, no DTD/external entities or excessive nesting |
| XLSX | Select a sheet with a header; export text cells, never formulas; continue to another sheet after 1,048,575 data rows | Built-in Excelize; styled dates become date/time strings |
| DB | Select a SQLite table; export a new SQLite file with TEXT/NULL columns | Built-in pure Go SQLite; virtual tables are not imported |
| XLS | Select an Excel 97–2003 sheet; write a real binary workbook | Windows x64 ACE; one sheet holds 65,535 data rows; sheet name max 31 characters and no whitespace or []:*?/backslash |
| MDB / ACCDB | Select an Access user table; create a new Access file/table | Windows x64 ACE; no programs, macros, relationships or indexes |
| DBF | Read dBASE tables; write dBASE IV | Windows x64 ACE; uppercase ASCII field names, max 10 bytes; system-codepage values max 254 bytes; no NULL/control characters; unrepresentable values stop export |
| ODBC | Enter a connection string, enumerate source tables; export to a new table | Windows ODBC and matching driver; destination must support create, parameterized writes and transactions |

ACE/ODBC drivers are not bundled or installed automatically. ACE may be supplied by a matching-bitness Microsoft Access Runtime. ODBC settings depend on the selected driver; the hidden connection-string field is kept only in the current window/operation memory. Windows adapters run through bounded PowerShell/.NET processes.

## Import

1. Choose the target MySQL/MariaDB table and source format.
2. Choose a file or enter an ODBC connection string. Select the source table/sheet for multi-table files.
3. Review field mappings and the first row. Auto-increment/generated columns are excluded; one destination field cannot be mapped twice.
4. Generate SQL and explicitly confirm before appending records. Changing the source/mapping invalidates the previous preview.

Import limits: 64 MB source file; decoded text data up to 8 MB, 10,000 rows, 1,024 columns and 200,000 cells; generated SQL up to 8 MB. Exceeding a boundary stops import rather than truncating. Import appends to an existing table, with types/constraints enforced by the server. Only transactional tables support rollback; inspect the result after errors or cancellation.

TXT/CSV/XLSX use `\N` for NULL and an empty field for an empty string. A literal marker gains one leading backslash: `\N` represents literal `\N`. Other values, including UNC paths, retain their content. XLS additionally uses `\E` for an empty string and escapes literal markers the same way. JSON/XML/SQLite/Access/ODBC use their own NULL representation.

JSON export example:

```json
{"columns":["id","note"],"rows":[["9007199254740993123",null]]}
```

Numbers in JSON object-array imports do not pass through floating-point conversion. Nested values remain JSON text. XML exports use `<data><columns><column>...</column></columns><rows><row><cell null="true"/>...</row></rows></data>`.

## Export

Export uses a system Save As dialog; cancellation before choosing a destination does not read the table. There is no application-wide total row, file-size or cell-count cap. Go streams rows directly into the writer instead of sending the entire table through the UI or using the limited query-preview interface. Errors/cancellation do not publish an incomplete file.

Transfer moves field names and values, not indexes, constraints, triggers or original MySQL types. SQLite/Access/Excel use text where possible to preserve exact large numbers. Use SQL dump for table structure plus data; it does not include routines/events/triggers.

TXT/CSV formula protection is enabled by default and prefixes potential spreadsheet formulas with an apostrophe. Disable it when original values must be re-imported. XLSX uses text cells. System-driver exports are read back and rejected if the format/driver changes precision, encoding, NULL or empty strings.

Direct/SSH export uses one streaming SELECT on one connection; transactional-table consistency is the statement's read snapshot. PHP uses ordered pagination through independent requests, so do not modify the source during export when consistency matters. Updated PHP scripts shrink batches when responses are too large; per-response and individual-value protocol boundaries remain. Cancellation, network failure, full disks and format capacity problems report an error.

XLSX automatically continues in new sheets when one fills up. XLS has a single sheet's native capacity; use XLSX/CSV/JSON for larger exports. System-driver export streams through temporary JSONL and writes rows one by one. Parameters and ODBC secrets travel via stdin, not process arguments. Readback checks columns, row counts and duplicate-preserving SHA-256 multiset digests without sorting/loading the whole table into memory.

Files are generated, closed and validated in a separate temporary directory before replacing the destination. Failure/cancellation preserves an existing file. ODBC creates a new destination table, never replaces one, and requests rollback on failure. Driver DDL can implicitly commit and leave part of a new table after cancellation/timeout; the app does not delete that table automatically because another session might have replaced it. Inspect the destination.

## Binary and spatial values

Tables containing BINARY/VARBINARY/BLOB, BIT and spatial columns can be exported. The server encodes these values before the text bridge:

- Binary and BIT: `0x` plus complete HEX, such as `0x00FF80`. Empty binary is `0x`; NULL stays distinct.
- Spatial: `SRID=4326;WKB=0x...`, retaining WKB geometry and SRID. MySQL's default SRID axis order is retained for round trips.
- Mapping back to binary/BIT generates validated binary literals. Spatial targets use `ST_GeomFromWKB` with the recorded SRID. Invalid encodings stop before SQL preview; the server checks geometry, target SRS and constraints.
- Mapping to a text field retains the encoding as text. Arbitrary text is never guessed to be binary/spatial content.

Files/destination data sources hold encoded text, not native BLOB/spatial columns. HEX is about twice the original byte size. Native format restrictions still apply, such as DBF's 254-byte value maximum; unsupported values fail explicitly. Auto-increment/generated fields remain excluded from writable import mappings.

## Platforms

TXT/CSV/JSON/XML/XLSX/SQLite core readers/writers are designed for Windows/Linux/macOS builds. Windows packages and experimental Linux/macOS cross-builds are produced; ACE/ODBC adapters remain Windows x64 only. Other platforms explicitly report system-driver formats as unavailable. Cross-compilation does not validate macOS/Linux desktop runtime behavior; see [release builds](builds.md).
