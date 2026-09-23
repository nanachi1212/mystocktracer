# 參與貢獻

感謝你願意改進 mystocktracer。

mystocktracer 是一套以 TWSE、TPEx 與 MOPS 等官方台股資料為基礎的研究工作台，重視資料正確性、來源可追溯性、資料新鮮度的誠實呈現，以及本機優先的使用方式。本專案不提供投資建議，也不做買賣推薦。

本專案由 [jundizhou/easy-stock](https://github.com/jundizhou/easy-stock) 衍生而來，原作者署名、Git 歷史與授權條款均完整保留。目前的來源盤點與授權狀態請見 [OSS 來源稽核](./docs/oss-provenance-audit.md)。

## 回報問題

請在本 repository 提交：

- Bug 與資料異常：<https://github.com/nanachi1212/mystocktracer/issues/new/choose>
- 一般使用與研究方法討論：<https://github.com/nanachi1212/mystocktracer/discussions>（若尚未開啟，請改用 Issue）

Bug 報告請盡量包含：

- mystocktracer 版本或 commit SHA；
- 作業系統（Windows / macOS / Linux 或原始碼執行）；
- 可穩定重現的步驟；
- 預期結果與實際結果；
- 涉及的台股資料來源（TWSE／TPEx／MOPS／ToAlpha 等）；
- 畫面上顯示的資料日期或 `as_of` 時間；
- 已去識別化的截圖或日誌。

**請勿提交** API Key、Cookie、Token、帳號資訊、持倉內容、本機資料庫檔案或其他個人資料。安全性問題請依 [安全政策](./SECURITY.md) 私下回報，不要開公開 Issue。

## 功能建議

目前採 OSS transition / maintenance 模式，接受既有行為修復、相容性與來源整理。`TWstockfor_tick-stock-panel` 是唯一長期台股主產品；新台股核心功能請優先送往該專案，不在此建立第二條功能主線。

功能建議請說明：

- 目前遇到的問題與研究流程；
- 使用情境；
- 期望結果（不必限定實作方式）；
- 對台股資料的影響；
- 是否需要新的資料來源。

## 台股資料來源貢獻

新增或修改資料來源時，請一併說明：

- 官方或公開文件連結；
- 使用條款與請求頻率限制；
- 是否需要登入；
- 欄位語意、單位與原始單位到正規化單位的轉換規則；
- 交易日期、`as_of`、資料新鮮度如何判定；
- 取得失敗時的降級行為；
- 若為第三方 fallback，必須標記為 fallback，不得偽裝成官方資料。

涉及歷史研究、財報、月營收、股利或回測時，必須遵守 Point-in-Time 原則：不得用今天才知道的資料回填過去；無可信的公開時間證據時，`available_at` 留 `null` 並標記 `data_insufficient`，不得自行推測。

## 資料正確性要求

- 無法取得的資料不得以 `0` 呈現；`available` / `stale` / `partial` / `unavailable` / 未查詢必須可區分。
- 保留來源識別（provider、source、retrieved_at、trade_date）。
- 不同官方 dataset 不得只因欄位名稱相似就互相覆蓋。

## 開發流程

1. Fork 後從 `main` 開短生命週期分支。
2. 依 [開發者文件](./docs/development.md) 準備 Node.js、Go 與桌面建置環境。
3. 保持改動聚焦，行為變更補上對應測試。
4. 提交前執行相關測試；跨前後端改動請執行完整測試。
5. 開 Pull Request，說明問題、方案、驗證結果與介面變化。

完整測試：

```bash
npm test
```

```bash
npm --workspace desktop test
```

僅建置前端：

```bash
npm run build:frontend
```

後端：

```bash
cd backend && go test ./... && go vet ./...
```

## Pull Request 要求

- 不提交金鑰、登入狀態、本機資料庫、執行快取或建置產物；
- 不無故重寫無關程式碼，不混入大規模格式化；
- 介面變更附上截圖，並涵蓋空狀態、錯誤狀態與窄螢幕版面；
- 資料來源變更說明來源、欄位語意、時效、失敗降級與使用限制；
- AI 輸出變更須保留「事實」與「模型判斷」的邊界，並標明風險與證據來源。

## 使用 AI 協助開發

本專案允許使用 Codex、Claude、Gemini 等 AI 工具協助開發，但貢獻者必須：

- 實際閱讀並理解自己送出的每一行程式碼；
- 對正確性負最終責任，不得以「AI 產生的」作為錯誤的理由；
- 確認產生的程式碼沒有來自授權不相容的來源；
- 不得貼入未經確認來源的程式碼、CSS、元件、測試或文件文字；
- 不得將金鑰、持倉、個人資料送進外部模型再貼回 repository。

在 Pull Request 中請說明是否有 AI 協助，以及你做了哪些人工驗證。

## 外部程式碼與授權

- 禁止直接複製授權不相容或授權不明的程式碼、素材與文件文字。
- 若引用相容授權的外部程式碼，必須保留原始著作權聲明與授權條款，並在 PR 中註明來源 repository、路徑與 commit。
- 參考其他專案的產品概念或 UX 想法可以，但不得複製其實作。

## 安全與隱私

- 不要在 Issue、PR、截圖或測試資料中出現 API Key、AI 供應商憑證、Cookie、Token、完整持倉或本機資料庫內容。
- 日誌請先去識別化再貼上。
- 新增會寫入本機檔案或資料庫的功能時，請說明寫入位置與內容敏感度。

## 授權

本專案目前採用 [PolyForm Noncommercial License 1.0.0](./LICENSE)，屬於 source-available 軟體，**不是** OSI 定義下的開源軟體，商業使用需另行取得授權。

提交貢獻即表示你有權提交相關內容，並同意被合併的貢獻依本專案當前授權條款發布。

本專案正在進行來源稽核與獨立化準備（見 [OSS 來源稽核](./docs/oss-provenance-audit.md)）。在該工作完成前，授權條款不會變更。
