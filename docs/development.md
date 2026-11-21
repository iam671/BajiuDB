# Development and contribution

**English** · [简体中文](development.zh-CN.md) · [Documentation](README.md)

## Toolchain

| Component | Repository version |
| --- | --- |
| Go | `go.mod` declares 1.26.0 and selects toolchain 1.26.11 |
| Node.js | Development baseline 22.20.0 |
| Wails | Go package 3.1.0 and JavaScript runtime 3.1.0 |
| Frontend | Vue 3, TypeScript and Vite; exact dependency resolutions are in `frontend/package-lock.json` |

Go can download the selected toolchain when allowed. Install the native Wails dependencies for your OS; Windows uses WebView2, macOS uses system WebKit and Linux needs its platform development libraries. The current checked packaging workflow is Windows x64; macOS/Linux desktop builds and execution remain unverified.

## Clone and run

```sh
git clone https://github.com/iam671/BajiuDB.git
cd BajiuDB
npm ci --prefix frontend
npm run build --prefix frontend
go run .
```

Alternatively clone `https://gitee.com/sixeventy-one/bajiu-db.git` and enter `bajiu-db`. Both repositories use `main`.

For frontend-only work, run `npm run dev --prefix frontend`. A browser preview cannot verify native desktop windows, credential storage or the Go bridge. The Go app embeds `frontend/dist`; build the frontend before running it. Frontend changes need rebuilding to update embedded resources.

## Build and package

On Windows PowerShell, from the repository root:

```powershell
./scripts/build.ps1
./scripts/package.ps1
```

`build.ps1` runs npm CI/build, generates Windows icon/version/manifest resources and builds the production EXE. It does not need a global Wails CLI. `package.ps1` adds Markdown guides, local assets, tunnel scripts, the project license, third-party notices/licenses and the MySQL driver source archive, then validates the packaged executable checksum.

Output names for **0.0.1**:

```text
build/bin/BajiuDB.exe
dist/BajiuDB-0.0.1-windows-x64/
dist/BajiuDB-0.0.1-windows-x64.zip
dist/BajiuDB-0.0.1-windows-x64.sha256
```

Packaging may use `BajiuDB-package.exe` as an intermediate build output. A successful rebuild replaces the same-version release rather than accumulating dated copies. A running old application is left open; close it before starting the new release. Connection configuration is not build output and must not be deleted.

Keep the product version at 0.0.1 until the maintainer explicitly requests an increase. `wails.json`, frontend package metadata and its lockfile must agree.

## Checks

```sh
go test ./...
go vet ./...
npx --prefix frontend vitest run --root frontend --exclude src/App.test.ts
npm run build --prefix frontend
```

`frontend/src/App.test.ts` has historical failures recorded in [VALIDATION.md](../VALIDATION.md); the focused suite above excludes it. `npm test --prefix frontend` runs the full suite including that file. Do not report the excluded file as passing.

Run Go root-package checks after the frontend build, not during it: Vite temporarily clears the embedded resource directory. Do not run frontend tests concurrently with packaging; `npm ci` replaces `node_modules`. A documentation-only change needs link/asset review, not a claim that runtime behavior was retested.

### Optional integration tests

Use a dedicated disposable MySQL instance with database `bajiu_test_db`, a `customers` table (`id` BIGINT primary key, `name` VARCHAR, `active` BOOLEAN) and an `active_customers` view. The test account needs read access; the TLS failure case expects an untrusted test certificate. Never use an existing business database or saved personal credentials.

```powershell
$env:BAJIU_MYSQL_TEST_ADDR = '127.0.0.1:33179'
$env:BAJIU_MYSQL_TEST_USER = 'bajiu_test'
$env:BAJIU_MYSQL_TEST_PASSWORD = 'replace-with-isolated-test-password'
$env:BAJIU_TEST_KEYRING = '1'
go test ./internal/... -count=1 -v
```

Setting `BAJIU_TEST_KEYRING=1` enables actual credential-store operations. MySQL-enabled tests also use a temporary SSH server. For the optional PHP check, deploy both tunnel scripts on an isolated test site and set `BAJIU_HTTP_TEST_URL` and `BAJIU_HTTP_TEST_TOKEN`; see [tunnel deployment](../tunnel/README.md). Integration cases without their required variables are skipped. Driver-specific transfer checks and their platform constraints are recorded in [VALIDATION.md](../VALIDATION.md).

## Source layout

| Path | Purpose |
| --- | --- |
| `internal/connections/` | Engine adapters, configuration, credentials, sessions and database operations |
| `internal/datafiles/` | File formats and streaming transfer |
| `app.go`, `*_app.go` | Go methods exposed to the desktop client |
| `main.go`, `native_dialogs.go` | Application and native-window lifecycle |
| `frontend/src/` | Vue UI, interaction logic and tests |
| `scripts/`, `tools/resourcegen/` | Build, packaging, notices and Windows resources |
| `docs/`, `tunnel/` | Markdown guides and PHP tunnel deployment |
| `licenses/`, `third_party_sources/` | Third-party distribution materials |

## Contribute

Read [AGENTS.md](../AGENTS.md) and [pitfalls](pitfalls.md). Open an issue or a focused PR targeting `main` on [GitHub](https://github.com/iam671/BajiuDB) or [Gitee](https://gitee.com/sixeventy-one/bajiu-db). Describe the problem, resulting behavior, checks performed and any unverified platform/database behavior. Update both language guides when behavior changes.

Do not commit credentials, connection configuration, private database data or build outputs. UI layout changes must preserve draft/save semantics. New engine work must declare its actual capabilities and avoid issuing another engine's SQL. The code is licensed under [Apache-2.0](../LICENSE); dependencies retain their own terms in [third-party notices](../THIRD_PARTY_NOTICES.md).
