<p align="center">
 <img src="./desktop/assets/easy-stock.png" width="112" height="112" alt="mystocktracer Logo" />
</p>

<h1 align="center">mystocktracer：台灣股票研究工作台</h1>

> mystocktracer 以原作者 jundizhou 的 [easy-stock](https://github.com/jundizhou/easy-stock) 為基礎；原作者署名、Git 歷史與 [PolyForm Noncommercial License 1.0.0](./LICENSE) 均完整保留。目前完成範圍請見 [台股化 Roadmap](./docs/taiwan-roadmap.md)。

<p align="center"><strong>以 TWSE、TPEx 與官方資料為基礎的台股研究桌面應用程式</strong></p>

<p align="center">
 從市場廣度、市場情緒與產業雷達，走到個股官方證據與確定性解讀。 <br />
 AI 研究採明確啟動，保留資料日期、來源、缺漏與限制，不提供股票推薦。
</p>

<p align="center">
 <a href="./docs/taiwan-roadmap.md"><strong>台股化進度</strong></a> ·
 <a href="#台灣市場核心能力">台灣市場能力</a> ·
 <a href="#專案狀態">專案狀態</a> ·
 <a href="./ROADMAP.md">完整產品路線圖</a> ·
 <a href="#上游項目與授權">上游項目與授權</a>
</p>

## 台灣市場核心能力

以下為目前已完成並有測試覆蓋的能力：

- **官方台股資料**：TWSE、TPEx 與 MOPS 的證券目錄、上市／上櫃行情、指數、三大法人、融資融券與基本面資料。
- **台股每日總覽 Dashboard**：今日市場、我的持股、自選股、事件提醒、研究更新與需要注意六個區塊，各區塊保留各自的 `as_of`。
- **自選股**：台股自選清單與有界的漲跌幅變化追蹤。
- **持倉**：持股、成本、未實現損益、權重與產業集中度。
- **公司事件提醒中心**：台股公司行動事件同步、提醒與已讀狀態。
- **個股研究**：官方證據導向的個股資料與確定性解讀。
- **AI 研究**：由使用者明確啟動的 grounded AI research，不自動呼叫模型。
- **研究歷史與前次比較**：保存歷次研究，並比對前後證據差異。
- **市場廣度、市場情緒與產業雷達**：TWSE、TPEx 與合併市場視角，依官方產業分類建立。
- **資料新鮮度與來源語意**：明確區分 available／stale／partial／unavailable／未查詢，無法取得的資料不會顯示為 0。
- **Pull Request 品質 CI**：後端、前端與桌面端測試與建置的自動檢查。

> 本產品不提供買賣推薦，也不做收益承諾。

## 專案狀態

mystocktracer 正在進行 **source provenance audit** 與 **independent OSS transition preparation**：盤點仍繼承自上游 easy-stock 的程式碼與素材，並規劃需要原創重寫或移除的部分。

目前授權仍為 **PolyForm Noncommercial License 1.0.0**。在來源稽核與替換工作完成前，本專案**不應被稱為 OSI 定義下的開源軟體**。

詳細盤點與 Phase B 替換計畫見 [OSS 來源稽核](./docs/oss-provenance-audit.md)。

> repository 內保留的 easy-stock A 股章節、路由與畫面屬上游／舊版功能，不是 mystocktracer 的台灣預設產品入口。


## 社群與貢獻

- 遇到錯誤或資料異常，請使用 [Bug 報告](https://github.com/nanachi1212/mystocktracer/issues/new/choose)，並盡量附上版本、重現步驟與資料日期。
- 有產品建議或資料來源需求，請提交 [功能建議](https://github.com/nanachi1212/mystocktracer/issues/new/choose)，說明使用情境與期望結果。
- 希望參與程式碼或文件建設，請先閱讀 [貢獻指南](./CONTRIBUTING.md) 與 [產品路線圖](./ROADMAP.md)。
- 涉及安全問題時，請依 [安全政策](./SECURITY.md) 私下回報，不要公開敏感細節。

---

## 上游項目與授權

mystocktracer 保留 easy-stock 原作者、Git 歷史與著作權資訊。 easy-stock 原創的後端、前端、桌面端及文件採用 [PolyForm Noncommercial License 1.0.0](./LICENSE) 授權。

- 允許個人基於學習、研究、實驗及其他非商業目的使用、修改與散佈，但必須保留授權條款及著作權聲明。
- 未經作者明確書面許可，不得用於任何直接或間接商業用途。
- 如需商業使用，請透過上游 [jundizhou/easy-stock](https://github.com/jundizhou/easy-stock) 聯絡原作者 jundizhou，取得個別商業授權。
- 第三方依賴、資料來源及隨附資料仍分別遵循其原始授權條款及服務條款。
- 完整法律條款以 repository 內的 [LICENSE](./LICENSE) 為準。

> 本專案屬於原始碼可用（source-available）軟體，並非 OSI 定義下的開源軟體。目前的來源盤點狀態見 [OSS 來源稽核](./docs/oss-provenance-audit.md)。

---

## 風險提示

> 本項目僅用於學習、研究和資訊整理，不構成任何投資建議、收益承諾或交易依據。市場有風險，AI 輸出和第三方資料也可能有延遲、遺漏或錯誤，請務必結合原始資訊獨立判斷並自行承擔決策結果。

<p align="center"><sub>Local first · Evidence based · Human in control</sub></p>
