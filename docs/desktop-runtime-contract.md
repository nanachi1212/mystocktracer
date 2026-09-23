# Desktop 可觀察行為契約（Phase B4）

基準：`0fbc351564943288a9c16aff151555978299d40a`（B3）。本文件依現有測試、產品 API 與封裝需求整理；不是上游實作的重述。完成狀態以驗證紀錄為準。

## 啟動與退出

- 產品名稱為 `mystocktracer`，Windows appId 為 `com.nanachi1212.mystocktracer`。
- 在啟動 backend、初始化 Electron session、寫入 log 或 settings 前選擇／遷移 userData。失敗必須停止，不能改用空目錄掩蓋失敗。
- backend 僅監聽 `127.0.0.1`；每次啟動產生新的隨機 token。renderer 僅在 health ready 後取得 backend URL 與 token，token 不進 log。
- 開發模式可使用 `go run ./cmd/server`；封裝模式缺少 `mystocktracer-backend.exe`（macOS 無副檔名）必須失敗。
- backend stdout/stderr 經遮罩且限制長度；health 有總逾時及單次請求逾時。啟動失敗也要清理 child。
- 退出與更新前先 flush renderer storage，再停止 backend 及其子程序，確認退出後才完成。

## Renderer 邊界

- `contextIsolation: true`、`nodeIntegration: false`、`webviewTag: false`，拒絕 webview、任意導航及新視窗。
- 唯一 bridge 為 `window.mystocktracer`；明確列出 backend config、runtime log、版本檢查／下載／安裝／發布頁／備份目錄及訂閱 AI 官方網址操作。
- 不暴露 raw IPC、任意 channel 或 filesystem。IPC 驗證 sender 為產品主頁；renderer log 有大小／頻率限制並遮罩。
- 訂閱 AI 網址使用既有精確 allowlist。更新來源固定為 `nanachi1212/mystocktracer` GitHub Releases，不接受設定或環境指定其它 feed。

## 更新與備份

- Windows 使用 electron-updater 與 SHA-512 metadata 驗證；下載完成後才可安裝。
- 安裝前停止寫入並建立經驗證的獨立備份。備份失敗不得呼叫 `quitAndInstall`。保留所有既有備份，不自動輪替刪除。
- 未簽署 macOS 版本提供發布頁手動流程，不宣稱已執行原生更新。
- 錯誤訊息不洩漏 token／API key／cookie／credential。失敗可回報並重試，不吞掉安裝失敗。

## 資料與封裝

- userData 優先序、COPY／驗證／activation、衝突及復原見 [identity migration](desktop-identity-migration.md)。
- 保留 `settings.json`、`taiwan-watchlist.db`（研究歷史／alerts 等表）、`taiwan-portfolio.db`、Hermes home/workspace 與 Electron Local Storage 中的 AI chat state；保留未知一般檔案及 logs。
- 所有新 executable、ZIP、installer、mac app bundle、icon 名稱使用 mystocktracer。
- 封裝包含 frontend、獨立 backend、Hermes 0.18.2、Hermes LICENSE 及 THIRD_PARTY_NOTICES。不得包含使用者資料、測試 fixture、DB、secret、logs 或 migration staging。
- root LICENSE 及歷史 attribution 不變；工程替換證據不等於法律重新授權權利。

## 驗收

必須執行 migration failure/security/SQLite 測試、backend/frontend/desktop 測試、Windows release verifier、最終封裝的 fresh/migration/conflict/idempotence/AI streaming/child cleanup smoke、安裝程式轉換驗證及兩輪完整 diff review。macOS 未實跑須標示 NOT_TESTED。
