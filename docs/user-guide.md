# User guide

**English** · [简体中文](user-guide.zh-CN.md) · [Documentation](README.md)

## Connections and groups

Use **连接 (Connection)** or Ctrl+N, choose a database engine, then fill in its settings. Test, save and open the connection. Saved connections can be edited, duplicated or deleted from their context menu. Create groups from **我的连接 (My connections)**; names are edited directly in the tree. Drag a connection into a group or back to the root. Deleting a group moves its connections back to the root.

Engine icons identify connection types. A green status dot indicates an open connection; gray indicates closed. Double-click an open connection or database to expand/collapse it. A single click selects a closed database without opening it. An unopened database's icon remains gray until opened.

The examples below describe the MySQL/MariaDB management workflow. For other engines, use their [native queries and supported objects](database-engines.md).

## Browse objects and DDL

Double-click a database to load its objects. Single-click a table to select it and make its DDL available; manually select the right-hand **DDL** tab to view it. Double-click a table to open its data tab. Selecting another table in the left tree does not automatically replace an already open working tab.

The object list supports Ctrl/Command multi-select, Shift ranges, select-all and drag selection. Right-click a selected table to keep that selection. Batch deletion lists the complete target table names for confirmation. Click empty space to clear the selection; right-click it for create/import/export/dictionary and related tools.

Closing a database closes its related table, design, query and console tabs after any required unsaved-work confirmation. Other databases and the server connection remain open.

## Edit table data

Grid view is the default. Use the icons after the pagination information to switch to form view. Data pages contain up to 1,000 records; scrolling is virtualized. Drag column boundaries to resize; widths/visibility are remembered for that connection, database and table.

1. Click a cell to select it; double-click to edit that cell.
2. Enter finishes editing, Shift+Enter inserts a newline and Esc cancels the current cell change. Finishing an edit only updates the draft.
3. Use **保存 (Save)** to write the draft, **新增 (Add)** to insert a row or **删除 (Delete)** to remove selected records. Tab/Shift+Tab moves through editable fields.
4. The cell context menu can copy the full value, generate copy-as SQL/tab-separated text, set an empty string/NULL, generate UUID or refresh. Copy-as SQL is not executed automatically.
5. Use **单元格编辑器 (Cell editor)** or the editing cell's **…** button for full text, calendars, typed values and JSON/HEX tools.

Existing-row editing requires a primary key or usable non-null unique key and the original record snapshot. Tables without one allow inserts but keep existing rows read-only. BIGINT/DECIMAL values are retained as text rather than rounded JavaScript numbers. Spatial values are read-only in the data editor.

**开始事务 (Start transaction)** uses a dedicated session for eligible direct/SSH InnoDB tables. After saving inside a transaction, explicitly commit or roll back. PHP tunnels do not support these cross-request transactions. Rollback cannot undo already committed changes or guarantee reversal of nontransactional/trigger side effects; refresh after errors or cancellation to inspect server state.

## Queries and SQL console

**新建查询 (New query)** opens a closable tab in the selected database. Execute selected SQL or the full text. Eligible direct-field, single-table SELECT results support editing through the data editor; joins, aggregates, expressions and uncertain source mappings remain read-only. Use the source table tab to add records. Ordinary query previews retain their 1,000-row / 8 MB display boundary.

**命令行界面 (SQL console)** opens a tab with the selected database as its default. Finish SQL with a semicolon and press Enter, or press Ctrl+Enter. Shift+Enter adds a newline, ↑/↓ navigate history, Ctrl+C stops and Ctrl+L clears output. `help`, `clear`, `exit` and standalone `USE`/`\u` are supported. Result summaries show elapsed request time in seconds with two decimals. History stays in memory; output has explicit display limits.

The console is an SQL interface, not a system shell. Each query/console submission uses an independent session; put session variables or a complete transaction in one submission rather than expecting state to carry over.

## Design tables

Choose **新建表 (New table)** or **设计表 (Design table)**. Work in the fields, indexes, foreign keys, triggers, options, comments and SQL-preview sections. Field properties follow the data type; text fields offer searchable charset/collation choices. New text fields initially follow the database/table defaults, while existing definitions retain their actual settings.

Adding fields can suggest a type/length from common names; explicit manual choices take precedence. Index fields can be selected, reordered and assigned prefix lengths/sort directions. The grid and lower property area can be resized without modifying the schema draft.

Save executes the design directly. An unnamed new table prompts for a name; deletion of a design item always needs confirmation. Saving keeps the current section open. If the server structure changed, the designer reloads it, stops the stale save and retains the previous draft SQL for inspection. DDL may partially complete; server errors show details instead of promising rollback.

## Import, export and database tools

Import selects a source format, source table/sheet and field mapping, then previews SQL before explicit confirmation. Export uses a system Save As dialog and streams all records without an application total-row/file-size cap. File-format and import limits still apply. Read the [format guide](data-transfer.md).

**转储 SQL 文件 (Dump SQL file)** offers structure/data or structure-only and opens Save As with `database.sql`. Dumps cover tables/views and optional rows; they are not a complete backup of routines/events/triggers. The data dictionary can be saved as HTML; a model based on declared keys can be saved as SVG. These are output artifacts; project guides remain Markdown.

Use **运行 SQL 文件 (Run SQL file)** for a reviewed SQL batch. Database deletion requires typing the exact database name. Closing the main window checks outstanding drafts, queries and transactions before exiting.
