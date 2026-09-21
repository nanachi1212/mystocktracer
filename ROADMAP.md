# mystocktracer 產品路線圖

mystocktracer 的目標是建立一套理解台股語境、證據可追溯、本機優先的研究工作台。路線圖用於表達優先方向，不代表固定發布日期；實際順序會依資料來源穩定性、使用回饋與維護成本調整。

本專案由 [jundizhou/easy-stock](https://github.com/jundizhou/easy-stock) 衍生。舊版 A 股 runtime 已於 Phase B1 移除；上游署名、歷史與目前授權仍保留。

---

## 已完成（台股產品）

| 能力 | 狀態 |
| --- | --- |
| Pull Request 品質 CI（後端／前端／桌面測試與建置） | 已完成 |
| 台股官方資料基礎（TWSE／TPEx 目錄、行情、指數、法人、融資融券、基本面） | 已完成 |
| 市場廣度、市場情緒、產業雷達 | 已完成 |
| 台股個股研究與確定性解讀 | 已完成 |
| 台股公司事件（corporate event）資料整合 | 已完成 |
| 事件提醒中心（Event Alert Center） | 已完成 |
| AI Research v2（使用者明確啟動、grounded） | 已完成 |
| 研究歷史（Research History） | 已完成 |
| 前次研究比較（Previous Research Comparison） | 已完成 |
| 台股自選股（Watchlist） | 已完成 |
| 台股持倉（Portfolio） | 已完成 |
| 台股每日總覽 Dashboard | 已完成 |
| 資料新鮮度語意（available／stale／partial／unavailable／未查詢） | 已完成 |

---

## 台股產品後續方向

### 資料可信度

- 持續強化資料來源健康檢查、快取、降級與交易日判斷；
- 為關鍵指標補上更新時間、來源、覆蓋率與異常說明；
- 減少第三方頁面或介面變動造成的靜默錯誤；
- 需要歷史研究的資料逐步補上 Point-in-Time 與 revision 語意。

### 研究閉環

- 強化個股證據、事件提醒與研究歷史之間的關聯；
- 讓歷史判斷、驗證結果與使用者研究方法可以長期累積；
- 改善 AI 研究的證據樹與結論驗證邊界。

### 桌面體驗

- 改善 Windows 與 macOS 的安裝、更新與故障診斷；
- 完善模型與資料來源狀態的可觀察性；
- 保持敏感設定、本機資料庫與研究紀錄的本機隔離。

---

## OSS Independence / Licensing Transition

本階段的目標是讓 mystocktracer 具備獨立維護的 repository identity，並為未來可能切換到 OSI 相容授權做好準備。授權條款在稽核與替換完成前不會變更。

### Phase A：來源稽核與 repository hygiene（已完成）

- 完整盤點仍繼承自上游 easy-stock 的程式碼、素材與文件；
- 分離「原創 mystocktracer 實作」與「仍依賴上游框架的部分」；
- 修正 repository identity（package 名稱、支援連結、貢獻文件、Issue／PR 模板）；
- 建立 [OSS 來源稽核文件](./docs/oss-provenance-audit.md)。

### Phase B1：Legacy A-share 與 supply-chain independence（已完成）

- 已移除 legacy A-share runtime、舊登入 bridges、上游 A 股文件／素材與 AGPL-only WeChat 元件；
- updater 已切換為 mystocktracer GitHub Releases。

### Phase B2：Core Runtime Independence（本輪完成）

- Go module、backend bootstrap、settings persistence、HTTP boundary 與 foundation 已原創替換；
- frontend API transport、Taiwan-only app shell 與 global styling foundation 已原創替換；
- `MYSTOCKTRACER_*` 成為 canonical runtime env，`A_STOCK_*` 僅保留 deprecated read fallback；
- Hermes AI 核心與 desktop identity 刻意留待後續階段。

### Phase B3：AI Runtime Independence（下一階段）

- 原創替換 Hermes model runtime、AI transport／chat UI 與高度相依的 settings UI；
- 保持既有 AI Research schema、tool behavior 與歷史資料相容。

### Phase B4：Desktop Identity Independence（後續）

- 原創替換 desktop shell、封裝 identity 與共用 logger；
- 先完成可回復的 userData migration，再評估 `app.setName`、`appId`、`productName`；
- 原創替換圖示與 favicon。

詳細清單、難度與相依順序見 [OSS 來源稽核 — Phase B replacement plan](./docs/oss-provenance-audit.md#phase-b-replacement-plan)。

### Phase C：授權評估

- 在 Phase B 完成後重新評估 HEAD 是否可切換授權；
- 保留必要的上游歷史署名。

---

## 舊版 A 股能力（Phase B1 已移除）

下列上游能力已不在目前 runtime tree：

- A 股行情總覽、指數、資金、產業、題材；
- 趨勢題材雷達、漲停梯隊、超短情緒；
- 雪球、淘股吧與微信公眾號文章收集與大 V 複盤；
- A 股持倉巡檢（portfolio inspection）；
- 游資心法資料庫（trading mastery）。

這些模組的處置方式（保留／替換／移除／待調查）記錄於 [OSS 來源稽核](./docs/oss-provenance-audit.md)。

---

## 如何參與

- Bug 與資料異常：使用 [Issue 模板](https://github.com/nanachi1212/mystocktracer/issues/new/choose) 提交可重現資訊；
- 功能建議：描述真實研究情境、目前阻礙與期望結果；
- 程式碼與文件：閱讀 [貢獻指南](./CONTRIBUTING.md) 後提交 Pull Request。

後端 API 路由見 [Backend API Routes](./backend/docs/api-routes.md)；台股化細節進度見 [台股化 Roadmap](./docs/taiwan-roadmap.md)。
