# 安全政策

## 回報安全問題

若你發現可能洩漏下列資料的安全問題，請**不要**建立公開 Issue：

- API Key 與 AI 供應商憑證（模型 API Key、Token、Cookie、登入狀態）；
- 本機資料庫（自選股、持倉、研究歷史、公司事件、複盤紀錄）；
- 使用者設定檔與環境變數檔；
- 執行日誌與快取內容。

請透過本 repository 的 **Security** 頁面建立私密安全報告：

<https://github.com/nanachi1212/mystocktracer/security/advisories/new>

若該入口暫時無法使用，請在 <https://github.com/nanachi1212/mystocktracer/issues> 建立一個**不含漏洞細節**的 Issue，請維護者提供私密聯絡方式。

## 報告內容

報告中建議包含：

- 受影響版本或 commit SHA，以及作業系統；
- 問題類型與可能影響；
- 最小重現步驟；
- 你認為可行的緩解建議。

**請不要在報告中附上**：

- API Key、Token、Cookie 或任何憑證原文；
- 完整持倉內容或帳戶資訊；
- 完整的本機資料庫檔案；
- 未去識別化的日誌或截圖。

若重現問題必須用到上述資料，請先去識別化（例如 `sk-****abcd`、以假資料取代持倉），或在私密管道中與維護者確認需要的最小資訊。

## 處理流程

維護者確認問題後會評估影響範圍、修復方案與發布安排。在修復版本可用前，請避免公開利用方式或敏感細節。

## 支援範圍

優先處理 `main` 分支與最新正式版本中可重現的問題。舊版本可能需要先升級到最新版本再驗證。

本專案由 [jundizhou/easy-stock](https://github.com/jundizhou/easy-stock) 衍生。若問題明確只存在於上游版本、且本 repository 已不含相關程式碼，請改向上游回報。
