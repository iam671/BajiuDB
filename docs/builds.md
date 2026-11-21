# Release builds

**English** · [简体中文](builds.zh-CN.md) · [Documentation](README.md)

Version **0.0.1** is available as these build targets. A successful cross-compilation is not a native desktop runtime test.

| Platform | CPU | Archive | Status |
| --- | --- | --- | --- |
| Windows | x64 | `BajiuDB-0.0.1-windows-x64.zip` | Existing native Windows production build |
| Linux | x64 | `BajiuDB-0.0.1-linux-x64.tar.gz` | Cross-compiled experimental purego backend; not launched on Linux |
| Linux | ARM64 | `BajiuDB-0.0.1-linux-arm64.tar.gz` | Cross-compiled experimental purego backend; not launched on Linux |
| macOS | Intel x64 | `BajiuDB-0.0.1-macos-x64.zip` | `.app` bundle, cross-compiled purego; no Developer ID signing/notarization or Mac runtime test |
| macOS | Apple Silicon | `BajiuDB-0.0.1-macos-arm64.zip` | `.app` bundle, cross-compiled purego; no Developer ID signing/notarization or Mac runtime test |

Each archive has a matching `.sha256` file. It includes both README languages, Markdown guides, screenshots, tunnel scripts, the project license and third-party distribution materials. `SHA256SUMS.txt` hashes the executable; Unix packages also include `BUILD-INFO.json` and a bilingual `README-platform.md`. macOS ZIPs and Linux tar archives preserve executable permissions.

## Runtime requirements

- **Windows:** Microsoft WebView2 Runtime.
- **Linux:** a graphical desktop, GTK4 and WebKitGTK 6.0. Runtime libraries are not included. Ubuntu 24.04/Debian 13 package names are `libgtk-4-1 libwebkitgtk-6.0-4`; install equivalent libraries on other distributions. This GTK4 package does not target Ubuntu 22.04/Debian 12. Remembered passwords need an unlocked Secret Service such as GNOME Keyring. Extract and run `./BajiuDB`.
- **macOS:** bundle deployment target is macOS 12+. Extract the ZIP and move `Bajiu DB.app` to Applications, choosing the archive for your CPU. These are test builds without an Apple Developer ID signature or notarization; macOS may block them. Signing/notarization and launch validation require a Mac and the appropriate signing identity.

The app and system-driver transfer features have different requirements: ACE/ODBC formats remain Windows-specific. See [data transfer](data-transfer.md). No package includes private credentials or user connection settings.

Wails 3.1.0 documents `CGO_ENABLED=0` and `-tags purego,production` for its experimental cross-platform backend. Native runtime libraries and packaging/signing remain separate requirements: [Wails release notes](https://v3.wails.io/blog/wails-v3-1-released/), [Linux packaging](https://v3.wails.io/guides/build/linux/).

## Rebuild all targets on Windows

Install the repository's Go/Node toolchain and **Python 3.11+**, then run from Windows PowerShell:

```powershell
./scripts/package-all.ps1
```

The script first uses the existing Windows build/package process, including frontend generation and license collection. It then builds four Linux/macOS binaries with `CGO_ENABLED=0` and `production,purego`, packages them and validates the binary type/CPU, archive executable hash and permissions. No C compiler, Docker or remote build service is required for this path. Python is a packaging tool, not an application runtime dependency.

To package previously built `.local/cross-build` binaries, use `-SkipCrossBuild`. `-SkipWindowsBuild` skips the Windows/frontend preparation and should only be used when those outputs and license materials are already current. Both options reuse existing outputs; they do not prove the binaries match later source edits.

Only fixed same-version output names are replaced, after the staged archive has passed validation. Other versions, connection settings and credentials are not cleanup targets. Desktop execution, code signing and database compatibility are separate from compile/archive checks; see [validation records](../VALIDATION.md).
