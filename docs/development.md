# mystocktracer 開發與封裝維護

本 repository 已完成 B6 技術來源獨立化，維護現行 MIT 程式碼。新的台股核心產品能力優先在 [TWstockfor_tick-stock-panel](https://github.com/nanachi1212/TWstockfor_tick-stock-panel) 實作；此處維護既有功能、資料相容性、測試與封裝。來源與授權決策見 [OSS 稽核](oss-provenance-audit.md)。上游 easy-stock attribution、Git 歷史與原授權依 [NOTICE](../NOTICE.md) 保留。

## 環境與安裝

使用 Node.js 22 以上、npm、Git，以及 backend/go.mod 指定的 Go。桌面 snapshot tests 需要 Python 3.11 以上；準備完整 Hermes runtime 另需 uv。安裝依賴執行：

```powershell
npm.cmd ci
```

完整桌面套件須在目標 OS／CPU 原生建置。Windows 使用 PowerShell；macOS 的 DMG 與 codesign 驗證須在 macOS 執行。實際 dependency versions 以 package-lock.json、backend/go.mod 與 runtime-manifest.json 為準，不因本輪 cleanup 升級。

## 啟動

一鍵建置並重啟 Web 工作台：

```powershell
npm.cmd run restart
```

預設 backend 為 http://127.0.0.1:20081、frontend 為 http://127.0.0.1:20073。runner 只停止 .runtime PID 記錄中可確認屬於本工作區的程序；遇到不屬於本工作區的 port listener 會報錯保留，不強殺。舊 scripts/rebuild-restart.sh 只是相同 Node runner 的相容入口。

需要熱更新時，在兩個終端分別執行：

```powershell
npm.cmd run dev:backend
```

```powershell
npm.cmd run dev:frontend
```

前端啟動後，另一個終端可執行 npm.cmd run dev:desktop。backend 與 desktop 的 npm dev scripts 透過 Node 傳遞環境變數，不依賴 POSIX 的 NAME=value 語法。Electron 會分配 loopback port 與一次性 token；開發環境缺少 desktop/bin/mystocktracer-backend.exe 時才使用 Go fallback。

## 設定與資料位置

| 設定 | 用途 |
| --- | --- |
| MYSTOCKTRACER_ADDR、MYSTOCKTRACER_TOKEN | 直接啟動 backend 的地址與 token |
| MYSTOCKTRACER_FRONTEND_HOST、MYSTOCKTRACER_FRONTEND_PORT | restart runner 的前端地址與 port |
| MYSTOCKTRACER_BROWSER_MODE | restart 的 incognito／normal／none |
| VITE_MYSTOCKTRACER_BACKEND_URL、VITE_MYSTOCKTRACER_TOKEN | Web 開發前端連線 |
| MYSTOCKTRACER_SETTINGS_PATH | 明確 settings 路徑 |
| MYSTOCKTRACER_TAIWAN_WATCHLIST_DB、MYSTOCKTRACER_TAIWAN_PORTFOLIO_DB | 既有資料庫 |
| MYSTOCKTRACER_CASHFLOW_CACHE、MYSTOCKTRACER_LOG_DIR | cache 與日誌 |
| MYSTOCKTRACER_HERMES_RUNTIME_ROOT、MYSTOCKTRACER_HERMES_PYTHON | runtime 與 interpreter |
| MYSTOCKTRACER_HERMES_HOME、MYSTOCKTRACER_HERMES_WORKDIR | agent 的設定與工作目錄 |
| MYSTOCKTRACER_USER_DATA_DIR | 桌面明確 profile，設定後不自動匯入舊目錄 |
| MYSTOCKTRACER_UPDATE_BACKUP_DIR | profile 外的更新備份位置 |
| MYSTOCKTRACER_DESKTOP_ARCH | 目標 CPU，必須符合 native runtime |
| MYSTOCKTRACER_PACKAGE_RESOURCES_DIR | desktop/dist 下的專用 staging 子目錄 |
| MYSTOCKTRACER_HERMES_RUNTIME_SOURCE | 可選的完整已驗證 runtime 複本來源 |

A_STOCK_* 與 VITE_A_STOCK_* 僅作既有環境的 deprecated fallback；canonical 名稱優先。ELECTRON_RENDERER_URL、UV 與 CSC_* 等為第三方工具介面，保留原名。

桌面預設使用 Electron appData/mystocktracer。舊 easy-stock／desktop profile 透過 COPY、hash 與 SQLite 驗證遷移，來源永久保留。直接 backend 依 canonical／easy-stock／a-stock-ai 順序採用有 settings、台股 DB 或 agent state 的 profile，不搬移資料。完整規則見 [migration 契約](desktop-identity-migration.md)。

Windows 更新會完整退出並重新啟動，在 Chromium／backend 尚未寫入前做備份，再驗證同一 release 後安裝；需要可用的 release 連線。macOS 未簽署版本採發布頁手動安裝。不要把使用者 profile 當 build output。

## 本機驗證

```powershell
npm.cmd test
npm.cmd --workspace desktop test
npm.cmd run test:tooling
npm.cmd run build:frontend
npm.cmd run build:backend
npm.cmd run audit:provenance
git diff --check
```

在 backend 目錄另執行 go vet ./... 與 go build ./...。現有線上測試分別以 EASY_STOCK_TW_LIVE_TEST=1 啟用 Taiwan provider、A_STOCK_LIVE_TEST=1 啟用 ToAlpha；這些是保留的測試相容旗標，預設不送出 live acceptance requests。本輪未新增 skip，也未將離線套件通過視為線上資料驗收。

## 桌面封裝

```powershell
npm.cmd run prepare:desktop-package
npm.cmd run package:windows
npm.cmd --workspace desktop run smoke:packaged-desktop
npm.cmd --workspace desktop run smoke:packaged-runtime
```

package:windows 已包含 preparation 與 verifier；單獨 prepare 僅用於診斷，不代表完成成品驗收。Windows 成品位於 desktop/dist/builder-dir/win-unpacked，執行檔必須為 mystocktracer.exe；backend 必須為 mystocktracer-backend.exe。staging 不允許指向 repository 根、.git、.runtime、來源目錄或 junction／symlink。

packaged-desktop smoke 使用合成 profile，驗證 PE metadata、canonical bridge、品牌圖片、migration，以及真實 localStorage 持有鎖與完整退出後的備份。packaged-runtime smoke 使用合成 agent runtime 驗證串流，不使用個人模型金鑰。

macOS 使用 npm run package:mac；release:mac／release:windows 會產生 installer artifacts，正式發布須由維護者明確操作。一般 packaging 呼叫 electron-builder 的 publish: never，不會自動發布。

## 來源與維護邊界

- 台股 market truth、PIT 與 canonical provider ownership 依 [ownership 文件](TAIWAN_DEVELOPMENT_OWNERSHIP.md)。
- dependency 使用各自授權；Hermes 為第三方 runtime，並非 inherited easy-stock implementation。
- GitHub PR Quality 執行 Go、frontend、desktop 與 tooling tests/build；GitHub Codex Review 處理實際阻擋 findings。
- 現行程式碼採 [MIT](../LICENSE)，歷史 upstream 與第三方內容維持原授權；此範圍來自維護者明確決策，技術稽核本身不授予重新授權權利。
