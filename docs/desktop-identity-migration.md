# Desktop identity 與資料遷移契約（Phase B4）

## 基準與選擇

B3 identity 為 `easy-stock`，另有較早的 `desktop` compatibility path。新 userData 為 Electron `appData/mystocktracer`，Windows 即 `%APPDATA%\mystocktracer`。

選擇順序：

1. 非空 `MYSTOCKTRACER_USER_DATA_DIR`，再讀取 deprecated `A_STOCK_USER_DATA_DIR`；明確設定時直接使用，絕不匯入別處資料。
2. canonical 已有 meaningful state 時直接使用，不從 legacy merge。
3. canonical 未初始化，優先 COPY `appData/easy-stock`；其次 `appData/desktop`。
4. 無舊資料才建立 fresh canonical。

meaningful state 包含任意非快取的一般檔案，不能只以 settings 判斷。canonical 只要非空但不能判定為 meaningful，也不得覆蓋。路徑中的 symlink／junction 拒絕；來源與目的不得互相包含。

## 交易式 COPY

先確認來源未被舊 desktop/backend 寫入，建立來源 inventory，將全部 legitimate state COPY 至同層唯一 staging。逐檔核對 SHA-256／大小及完整 inventory，再核對來源沒有變更；SQLite 檔案必須可讀且 integrity check 通過。保留 WAL／journal 等資料庫交易檔，不能把它們當 cache 刪掉。

只排除根目錄已知 Electron cache、兩代 updater cache 及 transient singleton process 檔案；不遞迴依名稱刪除使用者子目錄。保留 Local Storage、Session Storage、Partitions、未知產品檔、settings、DB、Hermes `.env`／model profiles／MCP secrets 與 logs。

manifest 僅包含 schema、source identity、UTC 時間與相對路徑／大小／hash，不包含檔案內容或完整來源路徑。成功後以 directory rename 啟用，目的地必須不存在或已證實為空。legacy 永久保留，本輪不刪除、搬移或改寫來源。

失敗 staging 保留供診斷但不作有效資料。下次從完整來源重建新的 staging，不自動信任部分複本、不刪除未知 staging。marker 隨完整 target activation 出現；canonical 已存在時重跑不覆蓋。來源改變、copy／verify 失敗、非空 target 競爭、link 或 DB 損壞均中止啟動，不啟動 backend／寫入 settings。

## Windows installer 決策與驗證要求

目前本機 `app-builder-lib 26.15.3` 的 `scheme.json` 確认 `nsis.guid` 是受支援選項；`NsisTarget.js` 預設以 appId 與固定 namespace 產生 UUID v5。直接換 appId 會改 installer registration，不能假定原地升級。

`templates/nsis/multiUser.nsh` 依 GUID 的 InstallLocation 決定舊位置；`installSection.nsh` 執行標準 NSIS replacement；`installUtil.nsh` 傳入 `/KEEP_APP_DATA`。`deleteAppDataOnUninstall` 必須為 false。若採保留 B3 GUID，需驗證新 executable／shortcut 與 registration 對應，不能自行寫 registry 清除或 uninstall 舊 app。

B3 `electron-updater 6.8.9` 的 `NsisUpdater.doInstall` 依 metadata 選 installer，傳入 `--updated`／`--force-run`，沒有硬編碼舊 exe 名稱；若明確設定 installDirectory 才增加 `/D=`。B4 版本仍是 0.9.2；升級驗證的較高版本只存在 fixture，實際 release 需要依 release policy 使用較高版本。

安裝轉換策略必須在 generated metadata／實際或隔離 fixture 驗證後定案。若無法證明安全，不得 merge 或標記 B4 completed。

參考：[electron-builder NSIS 文件](https://www.electron.build/nsis/)，並以 repository 已安裝版本的 schema 與 templates 為實作依據。
