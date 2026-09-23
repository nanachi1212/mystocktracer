# 桌面更新與發布維護

mystocktracer 使用 electron-updater，更新來源固定為本 repository 的
[GitHub Releases](https://github.com/nanachi1212/mystocktracer/releases)。
實作與測試分別在 update-feed.cjs 與 test/update-feed.test.cjs；
使用者設定或環境變數不能改變更新來源。

## 使用者資料與更新順序

- 啟動後 30 秒及每 12 小時檢查一次；下載、安裝均由使用者啟動。
- Windows 安裝前先寫入一次性更新意圖，完整退出目前 Electron，再重新啟動。
  新 process 在建立 BrowserWindow、session、backend 或更新網路連線之前，先建立經驗證的資料備份。
  flushStorageData 本身不會釋放 Windows 的 LevelDB / Cookie handles，不能代替完整退出。
- 備份成功後重新確認 GitHub 發布版本與使用者選定版本相同，再下載並交給原生 installer。
  因此重新啟動後仍需網路；離線、版本改變或備份失敗時不安裝，保留資料並正常啟動。
- 備份包含 settings、資料庫、未知 extension state、Hermes 與瀏覽器狀態。
  排除根目錄可重建 cache；複製後比對來源與目的 SHA-256，SQLite 需通過 integrity check。
- 外部 mystocktracer-update-backups 目錄保留所有成功備份，不自動淘汰歷史版本。
  失敗嘗試僅移除該次 .partial staging；不修改原始資料或已完成備份。
- 首次身份遷移依 [桌面資料遷移契約](../docs/desktop-identity-migration.md) 執行。
  新 profile 已有資料時優先使用；舊 easy-stock / desktop 來源保留，未知 state 不刪除。

macOS 未簽章套件使用手動下載流程。已簽章套件的自動安裝與 DMG 行為仍需在 macOS 驗證；
Windows 本機 smoke 不代表 macOS 或正式跨版本 installer 升級已通過。

## 歷史版本

Phase B1 之前的 easy-stock 客戶端仍使用上游更新來源。本 repository 無權修改其既有 feed，
這些安裝需要一次手動安裝 mystocktracer。這是保留舊名稱的相容性原因。

直接執行 NSIS 或拖入 DMG 不會走應用程式內的備份流程。先關閉應用程式，
保存完整 userData 外部副本，再安裝與確認資料。Windows 保留歷史 installer GUID，
以維持既有安裝登錄與升級路徑；新 appId、執行檔、產品名稱與素材屬於 mystocktracer。

## 發布維護

正式發布需維護者另行授權，本次 OSS cleanup 沒有推 tag、執行 release workflow 或發布套件。

Desktop Release workflow 接受 v* tag 或明確指定的既有 tag，每個 job 均 checkout 該 tag；
版本必須等於 desktop/package.json。品質檢查後，各原生 runner 建置 Windows x64、
macOS x64 與 arm64，再合併 metadata、驗證 SHA-512、產生 SHA256SUMS.txt 與發布。

- Windows：NSIS EXE、blockmap、latest.yml。
- macOS：各架構 ZIP、ZIP blockmap、DMG，以及合併的 latest-mac.yml。
- 不可從已發布版本移除 metadata 或其引用檔案。
- 簽章使用既有 WIN_CSC_LINK / WIN_CSC_KEY_PASSWORD 或 MAC_CSC_LINK / MAC_CSC_KEY_PASSWORD。
  Apple notarization 使用既有 APPLE_ID / APPLE_APP_SPECIFIC_PASSWORD / APPLE_TEAM_ID。
- 無簽章的本機套件只可作已聲明範圍的 smoke；不代表正式簽章發布已驗證。
- package resources override 僅允許 desktop/dist 專用子目錄，拒絕來源、Git metadata、
  runtime state、祖先與 junction / symlink；prepare 會替換該專用產物。

本機檢查可執行：

```powershell
npm.cmd run package:windows
npm.cmd --workspace desktop run smoke:packaged-desktop
npm.cmd --workspace desktop run smoke:packaged-runtime
```

第三方 runtime、Electron 與相依套件保留各自授權，見
[THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md)。它們不是 easy-stock 繼承程式碼的同義詞。
