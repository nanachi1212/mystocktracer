# mystocktracer Repository Instructions

本檔只補充 mystocktracer 的 repository-specific 規則。
一般開發流程、Git/GitHub、安全、驗證與回報方式遵循使用者層 Global `AGENTS.md`。

## Repository Identity

- `origin/main` 是 mystocktracer 的 canonical product branch。
- `upstream/main` 僅作為 easy-stock 上游參考。
- 未經明確授權，不要把 `upstream/main` 直接 merge、rebase 或整批同步進本 repository。
- 保留 easy-stock 原作者、Git 歷史、現行授權與第三方來源資訊。

## Product Status

- 本 repository 目前定位為 OSS transition / maintenance 與既有功能參考來源。
- `TWstockfor_tick-stock-panel` 是唯一長期台股主產品與最終使用介面。
- 新的台股核心能力、量化選股、research、portfolio、alerts 與 dashboard 功能應優先在 TWStock 主產品實作。
- 不在 mystocktracer 建立新的平行產品主線，也不要為了維持兩個長期產品而新增 shared-dataset adapter。
- 若要移植 mystocktracer 的既有功能或 UX，應在 TWStock 重新適配並建立單一 authoritative implementation。

## Maintenance Boundaries

本 repository 的修改優先限於：

- 修復既有 bug
- 保護既有使用者資料與相容性
- source provenance audit
- OSS transition / independent rewrite preparation
- 為移植到 TWStock 提供可驗證的既有行為與契約參考
- 必要的安全、維護與依賴相容修正

若需求其實是新增長期台股產品能力，先判斷是否應改到 `TWstockfor_tick-stock-panel`，不要在此重複實作。

## PIT and Provenance Safety

- 保留 Point-in-Time 語意，禁止 future-data leakage。
- 保留市場資料與 derived facts 的來源、timestamp、scope、`as_of` 與 provenance。
- 不把 unavailable / stale / partial 資料偽裝成正常最新資料。
- 技術獨立化、來源替換與 relicensing 的實際 gate 以 `docs/oss-provenance-audit.md` 為準；不得因某段程式已重寫就自行宣稱整個專案已完成獨立化或已可 relicensing。
- 涉及來源 ownership、PIT、provenance 或授權時，優先讀取對應 audit / ownership 文件與實際 Git history。

## Migration and Reuse

- 維護既有行為時，優先重用成熟 provider、contract 與可驗證事實。
- 不刪除或重寫成熟 provider，除非該任務本身就是明確 migration / replacement。
- migration 必須保護 fallback、既有 user data、相容性與 recoverability。
- 從本 repository 移植功能到 TWStock 時，不要把 legacy architecture、授權受限程式碼或不必要的相依一起複製。

## Context Routing

不要每次工作都讀完整 repository 文件。

- source provenance / OSS transition / relicensing：讀 `docs/oss-provenance-audit.md` 與相關來源記錄。
- Taiwan ownership、PIT 或既有資料契約：讀實際相關 ownership / market 文件。
- upstream comparison：只在任務需要時查看 `upstream/main` 或上游來源。
- 一般 maintenance bug：只讀受影響調用鏈、測試與必要文件。
