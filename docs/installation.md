# Installation and configuration

**English** · [简体中文](installation.zh-CN.md) · [Documentation](README.md)

## Windows x64

Open the [GitHub releases](https://github.com/iam671/BajiuDB/releases) or [Gitee releases](https://gitee.com/sixeventy-one/bajiu-db/releases) page. If an archive has been published, download `BajiuDB-0.0.1-windows-x64.zip` and its `.sha256` file, extract it and run `BajiuDB.exe`. If no binary is attached, use the [source build](development.md).

The application uses the system Microsoft WebView2 Runtime, which is not bundled in the ZIP. If it is missing, install it from [Microsoft](https://developer.microsoft.com/en-us/microsoft-edge/webview2/). Running the packaged app does not require Go or Node.js. The current UI language is Simplified Chinese.

To compare a downloaded archive against its published checksum in PowerShell:

```powershell
Get-FileHash -Algorithm SHA256 ./BajiuDB-0.0.1-windows-x64.zip
Get-Content ./BajiuDB-0.0.1-windows-x64.sha256
```

For updates, finish or discard unsaved work, close the old app and open the new package. Build outputs are separate from user connection settings.

## Other platforms

Experimental Linux x64/ARM64 and macOS Intel/Apple Silicon packages are cross-compiled using Wails purego. They passed compile/archive checks, but have not been launched on those systems. Linux requires GTK4/WebKitGTK 6.0; macOS packages are unsigned/unnotarized `.app` bundles. See [release builds](builds.md) for exact filenames and runtime requirements, or [development](development.md) to build locally.

## Connection settings

Select **连接 (Connection)**, choose an engine, enter a connection name and the engine's required fields, then test and save. Database type stays fixed for a saved connection. SQLite opens an existing regular file; it does not silently create a new one. See [engine settings](database-engines.md).

Saved passwords use the operating system credential store. Leaving a saved connection's password field empty preserves its existing credential; explicitly removing the saved password takes effect after saving the configuration. A password entered without remembering it stays only in the current application process.

## Configuration and credentials

Connection/group configuration is stored under `os.UserConfigDir()/BajiuDB/connections.json`:

| OS | Typical path | Credential store |
| --- | --- | --- |
| Windows | `%APPDATA%/BajiuDB/connections.json` | Credential Manager |
| macOS | `~/Library/Application Support/BajiuDB/connections.json` | Keychain |
| Linux | `$XDG_CONFIG_HOME/BajiuDB/connections.json`, usually `~/.config/BajiuDB/connections.json` | Secret Service |

The credential service name is `BajiuDB.connections`. JSON contains connection parameters and credential references, not plaintext passwords. Backing up the JSON does not migrate passwords to another machine. If the system credential service is unavailable, use the connection without remembering the password; the app does not fall back to plaintext storage.

Connection addresses, names and usernames are still configuration data. Do not include your personal configuration file in a public issue or commit. The app has no built-in cloud sync, telemetry or automatic updater.

## File-format dependencies

TXT/CSV/JSON/XML/XLSX/SQLite DB use built-in readers/writers. XLS/MDB/ACCDB/DBF require Windows x64 ACE; ODBC requires the matching system driver. These drivers are not bundled or installed automatically. See the [format guide](data-transfer.md) before importing or exporting.
