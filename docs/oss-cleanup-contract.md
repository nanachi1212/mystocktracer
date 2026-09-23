# OSS cleanup 驗證契約（2026-09-23）

本輪僅整理 packaging、tests、helper 與 provenance，不新增產品能力，也不修改 market truth、PIT 或 provider。原有功能的行為與必要 migration 保留。

## 測試重寫的行為對照

同路徑重寫測試，而不是靠改名宣稱原創。新的 fixtures 為合成資料，所有暫存目錄使用 test cleanup；既有行為逐項保留並補負向情境。

| 檔案 | 保留的驗證 | 新增或加強 |
| --- | --- | --- |
| desktop/test/backend-process.test.cjs | env、可用 port、指定 binary、開發 Go fallback | packaged 缺 binary 必須失敗、佔用 port、外部 health endpoint |
| desktop/test/hermes-runtime-root.test.cjs | explicit / bundle / worktree runtime | Windows 與 macOS 同一 precedence matrix、缺 runtime |
| desktop/test/hermes-runtime.test.cjs | Windows standalone Python、source link、stdlib 與 site-packages | macOS layout、來源重疊與缺件、來源不變 |
| desktop/test/runtime-logger.test.cjs | 寫入前 secret 遮罩、rotation、query context | mirror 同樣遮罩、每檔 byte 上限、精確最新 records、zero-backup |
| desktop/test/data-protection.test.cjs | settings / DB / secrets / unknown state / browser partition bytes、根 cache 排除、所有歷史備份、nested target 拒絕 | 來源 bytes 同時不變、相同路徑拒絕 |
| desktop/test/update-manager.test.cjs | updater 狀態、stop → backup → install、備份失敗、開發停用、macOS manual、signature error、真實 snapshot helper | 進度上限、downloaded 狀態不可被 check 清除 |
| desktop/test/updater-preload.test.cjs | IPC channels、subscription forwarding、listener cleanup、官方 URL allowlist | canonical frozen bridge 完整 key set、無 generic IPC、取消後不再通知 |
| frontend/src/components/AppUpdatePanel.test.ts | check / download / install / manual release | 所有 state × installMode matrix |
| backend/internal/runtimelog/writer_test.go | redaction、query context、rotation、persisted secrets | bytes bound、zero-backup、closed writer、path traversal |

B4 的 migration、packaged desktop/storage smoke、restart-before-backup tests 與 asset reference regression 均保留。沒有 skip、xfail、continue-on-error，也沒有刪除有效 scenario 來降低門檻。

## 包裝與開發入口

- 舊 Bash restart helper 只轉交既有 Node runner；不再維護獨立的 PID／port-kill／shell 拼接路徑。
- Node runner 優先採用 MYSTOCKTRACER_*，A_STOCK_* 僅作已存在工作環境的 fallback；向 backend／frontend 傳入 canonical 變數。
- package resource staging 只能在 desktop/dist 的專用子目錄；來源、Git、.runtime、祖先與 junction／symlink 均不可用。
- 相依軟體與工具不因曾被 easy-stock 使用而列為 inherited implementation。

## Provenance 證據

審核決策保存於 oss-provenance-decisions.json：每筆包含 path、正規化 CRLF 的 SHA-256 與 contract／phase 證據。內容改變但未更新證據時，不會繼續套用同路徑 original override。新版 audit 比對全部 tracked 與未忽略新檔案，對 fork 做跨路徑相同內容、Git rename/copy 與長語句重疊掃描；涵蓋 code、config、Python、Shell、tests、assets、docs 與 dependency lockfiles。

這不是法律 clean-room 證明。Git 新檔案與內容相似度工具不能自行證明著作權歸屬；來源分類、technical gate 與 LICENSE 變更決策必須分開。剩餘檔案與 gate 以 oss-provenance-audit.md 為準。

發布 workflow 依實際三個原生目標重新建立 matrix，共用品質檢查與 artifact 組裝；手動發布會 checkout 指定 tag。這是 workflow 原始碼驗證，未觸發正式發布或聲稱 macOS installer 實機驗證完成。
