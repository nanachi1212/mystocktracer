# Taiwan Portfolio Consumer Workflow

## Ownership

- `mystocktracer` 擁有使用者持倉、股數、平均成本、備註、組合呈現與消費者層集中度。
- Taiwan canonical identity、證券名稱、產業與行情來自既有 Taiwan directory / market consumer path；本功能不建立第二套市場資料庫。
- Portfolio 與 Watchlist 是獨立產品資料；刪除任一方不會連動刪除另一方或 alerts history。

## Persistence

`taiwan-portfolio.db` 的 `taiwan_portfolio_holdings` 以 canonical symbol 為唯一鍵，儲存 shares、average cost、optional note、created / updated time，以及僅作為 directory 不可用時顯示的 name snapshot。名稱快照不是 canonical company master，API 會優先使用當前 directory identity。

SQLite 使用單連線、busy timeout、非 memory DB 的 WAL，並以 transaction 執行加法型 migration。舊表若缺少 display name、note 或 timestamps，啟動時補欄與回填時間，不重寫現有數值。

## APIs

- `GET /api/v1/tw/portfolio`
- `POST /api/v1/tw/portfolio`
- `PUT /api/v1/tw/portfolio/{symbol}`
- `DELETE /api/v1/tw/portfolio/{symbol}`
- `GET /api/v1/tw/portfolio/summary`

寫入前必須通過後端 Taiwan directory 解析，shares 必須大於 0，average cost 必須非負，數值必須 finite，note 上限 500 字元。現有 directory 支援普通股與 ETF，Portfolio 不另行拒絕 ETF。

## Calculations and enrichment

內部計算保留 number，僅前端顯示時格式化：

- total cost = shares × average cost
- market value = shares × current price
- unrealized P/L = market value − total cost
- unrealized P/L % = unrealized P/L ÷ total cost × 100（total cost 為 0 時不計算）
- weight = holding market value ÷ total portfolio market value × 100

現價每次由既有 Taiwan market provider 取得，不寫入 Portfolio DB。任一持股行情 unavailable 時，該筆不以 0 代替，整體 market value、P/L、weight、Top 3 / Top 5 與產業集中度改為資料不足；`available_market_value` 僅表示已成功取得行情的小計。過期行情會標示 `stale`。

## Product integrations

- Watchlist 可帶入 Portfolio 新增表單；Portfolio 持股也可加入 Watchlist。canonical symbol 分別由兩邊唯一鍵防止重複。
- Portfolio workspace 每次 mount / 手動 refresh 後，將其中普通股以 explicit symbol list 交給既有 corporate-event sync。同步延用原 dedupe、baseline 與 alerts inbox，不輪詢、不建第二套 alerts，provider failure 不影響持倉。
- 持股列直接導航到既有 Taiwan Stock Research workspace，重用 AI Research、Research History 與 Previous Research Comparison。

## Limitations and legacy migration

本 MVP 不包含 cash ledger、已實現損益、股利、稅務、多幣別、券商匯入、下單或投資建議。集中度僅呈現單一持股、Top 3 / Top 5 與 directory industry 的描述性統計，沒有任意風險門檻。

舊 `portfolioinspection` 保留原樣；其 A 股代碼、權重設定、簡中語意與買賣動作規則不會自動搬入台股 Portfolio。未來若要 retirement，應先定義明確轉換規則與使用者確認流程。
