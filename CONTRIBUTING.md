# 參與 mystocktracer 維護

感謝提供可重現的修正。mystocktracer 目前以 OSS 來源整理與既有台股工作流維護為主；新台股核心能力應優先放到 `TWstockfor_tick-stock-panel`。本專案由 [jundizhou/easy-stock](https://github.com/jundizhou/easy-stock) 衍生，目前授權與來源狀態見 [LICENSE](LICENSE) 及 [OSS 稽核](docs/oss-provenance-audit.md)。

## 回報與修改

- Bug 請附版本、平台、重現方式、預期／實際結果。資料問題再附來源、交易日與畫面 `as_of`；安全問題走 [私密通道](SECURITY.md)。
- 從 `main` 建短期分支，將修改限制在具體問題。PR 說明行為契約、測試結果、資料與相容性影響。
- 介面變更請確認空、載入、錯誤狀態及窄螢幕。不要把大量重構或無關依賴升級混入修正。
- 不要提交憑證、Cookie、持倉、本機資料庫、執行日誌、快取或生成安裝包。

## 台股資料契約

TWstock 與交易所官方資料持有 canonical 台股事實；mystocktracer 處理研究與呈現。修改 provider、正規化或 Point-in-Time（PIT）邏輯前，先閱讀 [權責文件](docs/TAIWAN_DEVELOPMENT_OWNERSHIP.md)。每筆資料應保留來源、擷取／交易日期、單位與可用狀態。第三方資料只能以明確標記的補充或 fallback 進入；失敗與缺值不可偽裝成零。

歷史資料不能使用當時尚未公開的值。若無可靠公告時間，保留 `available_at=null` 並標示資料不足。新增來源請記錄原始欄位、單位轉換、請求限制、降級方式及可重現測試。AI 研究只能由使用者明確啟動，並保持事實與模型判斷的邊界。

## 驗證與來源

依改動執行相關 Go、前端或桌面測試；跨邊界變更請執行完整套件。命令與環境見 [開發文件](docs/development.md)。測試資料請使用合成值；有效測試不得為了通過 CI 而弱化或刪除。

不要複製授權不明或不相容的程式碼、文件與素材。若引用允許使用的外部來源，PR 必須標明 repository、commit、路徑、授權與保留的署名。使用 AI 工具協助時，提交者仍須審閱內容並對正確性負責。

目前貢獻依本 repository 的 [PolyForm Noncommercial License 1.0.0](LICENSE) 發布；PR 不構成授權變更。
