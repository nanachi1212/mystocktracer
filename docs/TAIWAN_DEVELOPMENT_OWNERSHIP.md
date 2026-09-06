# mystocktracer Development Ownership & Cross-Project Reuse Rules

## 1. Project Role

`mystocktracer` 的長期定位是：

> **Investor Product & Research Experience Layer**
>
> 投資者主產品、互動介面、追蹤、篩選、比較、通知、研究工作流與 AI 解讀層。

本專案的核心責任不是重新建立另一套完整台股資料工程，而是把可信資料轉換成：

- 可閱讀；
- 可搜尋；
- 可篩選；
- 可追蹤；
- 可比較；
- 可收藏；
- 可提醒；
- 可研究；
- 可由 AI 解讀；
- 可支援實際投資研究流程

的產品體驗。

---

# 2. Related Project

與本專案相關的另一個專案：

`nanachi1212/TWstockfor_tick-stock-panel`

兩專案必須視為同一生態中的不同責任層，而不是兩套互不相干的台股系統。

基本分工：

## TWstock owns Taiwan market truth

TWstock 原則上負責：

- Security Master
- TWSE / TPEx 官方資料
- MOPS 正規化資料
- Daily OHLCV
- Institutional Flow
- Margin / Short
- Monthly Revenue
- Financial Statements
- Point-in-Time Fundamentals
- Revision history
- Dividend lifecycle
- Valuation
- Share-capital records
- ETF / security classification
- Taiwan trading rules
- Official source provenance
- Data reconciliation
- Historical reconstruction
- Backtest-safe datasets
- Data quality
- Technical factors
- Fundamental factors
- Research Facts
- Deterministic market-derived metrics

## mystocktracer owns product experience

mystocktracer 原則上負責：

- Dashboard
- Home / Market Overview
- Watchlist
- Portfolio
- Stock detail UX
- Screener UX
- Saved screen / Saved query
- Compare workflow
- Market radar
- Industry / thematic discovery
- Alerts
- Notifications
- User preferences
- AI Research
- AI report presentation
- Research workflow
- Multi-market product integration
- User-facing error / stale / partial-data handling
- Product-level caching where appropriate

---

# 3. Mandatory Anti-Duplication Check

在實作任何台股相關新功能前，AI Agent 必須先執行「跨專案重複能力檢查」。

只要任務涉及以下任一類別，就不得直接開始重寫資料層：

- TWSE
- TPEx
- MOPS
- FinMind
- Yahoo market data
- Security Master
- Daily prices
- K-Line
- Institutional
- Margin
- Short selling
- Revenue
- Fundamentals
- Financial statements
- Dividend
- Valuation
- Share capital
- ETF metadata
- Technical indicators
- Fundamental factors
- Historical data
- Point-in-Time
- Research Facts
- Screener factors
- Backtest inputs
- Unit normalization
- Source provenance
- Availability
- Revision handling

必須先回答：

1. TWstock 是否已經有這個能力？
2. mystocktracer 是否已經有可用實作？
3. 哪個專案是 canonical owner？
4. 是否可以直接消費 TWstock 的共享資料？
5. 是否只需要新增 adapter，而不是重新做 provider？
6. 是否需要 fallback，而不是 primary implementation？
7. 是否會因此產生第二套資料口徑？

若 TWstock 已有成熟 canonical implementation：

> mystocktracer 不得建立第二套等價主實作。

優先順序必須是：

```text
Reuse shared dataset / contract
        ↓
Add local adapter
        ↓
Add compatibility layer
        ↓
Use existing mystocktracer provider as fallback
        ↓
Only if no reusable source exists:
implement new capability
```

---

# 4. Existing mystocktracer Providers Must Not Be Deleted Blindly

目前 mystocktracer 已經存在 Taiwan provider、MOPS、TWSE、TPEx、FinMind 等相關實作。

不要因導入 shared data 就直接刪除。

這些既有 provider 可重新定位為：

- fallback；
- verification；
- recovery；
- bootstrap；
- development diagnostics；
- shared dataset 尚未準備完成時的官方來源。

預期流程：

```text
mystocktracer
      │
      ▼
Shared Taiwan Market Data
      │
      ├── available / fresh
      │       ↓
      │      use
      │
      └── unavailable / unsupported
              ↓
      existing official provider
              ↓
      TWSE / TPEx / MOPS
```

不得在沒有 migration plan 與測試的情況下移除 working provider。

---

# 5. Canonical Ownership Rule

每個 domain 必須只有一個主要 owner。

目前預設：

```text
security_master              -> TWstock
daily_market_data            -> TWstock
institutional                -> TWstock
margin_short                 -> TWstock
monthly_revenue              -> TWstock
financial_statements         -> TWstock
point_in_time_fundamentals   -> TWstock
valuation                    -> TWstock
dividend_lifecycle           -> TWstock
security_rules               -> TWstock
historical_data              -> TWstock
technical_factors            -> TWstock
fundamental_factors          -> TWstock
research_facts               -> TWstock
backtest_data                -> TWstock
data_quality                 -> TWstock

dashboard                    -> mystocktracer
market_overview              -> mystocktracer
watchlist                    -> mystocktracer
portfolio                    -> mystocktracer
stock_detail_ux              -> mystocktracer
compare_workflow             -> mystocktracer
alerts                       -> mystocktracer
notifications                -> mystocktracer
saved_screener               -> mystocktracer
product_screener_ui          -> mystocktracer
market_radar_ui              -> mystocktracer
industry_radar_ui            -> mystocktracer
ai_research_ui               -> mystocktracer
user_preferences             -> mystocktracer
multi_market_product         -> mystocktracer
```

---

# 6. Existing mystocktracer-Owned Data Capability Exception

如果 mystocktracer 已經有某個明顯成熟、穩定且重建成本高的資料能力，而 TWstock 尚未有等價成熟實作，例如：

```text
MOPS XBRL cashflow ingestion
large archive discovery/download/parser pipeline
```

則不要為了 ownership 表「看起來漂亮」而立刻搬走。

處理原則：

```text
existing mature mystocktracer implementation
        ↓
keep owner temporarily
        ↓
publish normalized shared output
        ↓
TWstock consumes shared output
```

之後若真的需要轉移 ownership，再另開 migration phase。

禁止直接重寫第二套。

---

# 7. Data Truth vs Product Opinion

mystocktracer 必須清楚區分：

## A. Market Facts

例如：

```text
close
volume
revenue_yoy
eps
pe
pb
foreign_net_5d
margin_change_5d
relative_volume
rs_20d
dividend_yield
```

這些是 deterministic facts / factors。

如果 TWstock 已經提供：

> 直接使用。

不要自己重新計算。

---

## B. Product Opinion

例如：

```text
buy_score
watchlist_priority
risk_label
market_heat_score
ai_rating
bullish_score
user_relevance_score
```

這些屬於 mystocktracer 自己的產品邏輯。

允許自行設計、修改與演進。

原則：

> Share facts, not opinions.

---

# 8. Screener Ownership

mystocktracer 擁有產品型 Screener。

主要責任：

- 條件選擇；
- 使用者可理解的欄位名稱；
- AND / OR 篩選邏輯；
- 排序；
- pagination；
- saved filters；
- watchlist integration；
- stock detail navigation；
- AI explanation；
- UI states；
- empty / stale / partial handling。

但 Screener 使用的市場因子：

```text
revenue_yoy
pe
pb
foreign_net_5d
foreign_net_20d
margin_change_5d
rs_20d
relative_volume
```

若存在 canonical shared value：

> 不得在 mystocktracer 再建立第二個公式。

---

# 9. Market Radar Ownership

mystocktracer 可以擁有：

- Market Radar
- Industry Radar
- Sector Radar
- Theme Discovery
- Hot stocks
- User-facing market breadth presentation

但要拆開：

```text
Raw / normalized metrics
        ↓
Shared / TWstock

Product ranking / display / grouping
        ↓
mystocktracer
```

例如：

```text
advancing_count
declining_count
new_high_count
relative_volume
institutional_net
industry_return
```

如果是 deterministic underlying metric，優先共用。

但：

```text
hotness_score
recommended_sector
featured_stock
radar_priority
```

可以由 mystocktracer 自己決定。

---

# 10. AI Research Ownership

mystocktracer 擁有 user-facing AI Research experience。

包括：

- Research page
- Research action
- prompt orchestration
- report format
- explanation style
- summary
- comparison
- citation presentation
- user interaction
- follow-up questions
- report history

但禁止重新建立另一套底層 Taiwan Research Facts pipeline。

理想流程：

```text
Shared Research Facts
        ↓
mystocktracer AI context builder
        ↓
AI model
        ↓
user-facing research report
```

Research Facts 應包含類似：

```json
{
  "symbol": "2330.TWSE",
  "as_of": "...",
  "price": {},
  "technical": {},
  "institutional": {},
  "margin": {},
  "revenue": {},
  "fundamentals": {},
  "valuation": {},
  "dividend": {},
  "cashflow": {},
  "provenance": {},
  "capabilities": {}
}
```

mystocktracer 可以：

- 選擇哪些 facts 放進 prompt；
- 壓縮；
- 排序；
- 轉成人類可讀文字；
- 根據使用者問題選 context。

但不得偷偷重抓一套相同市場資料作為主要來源。

---

# 11. AI Must Respect Missing Data

AI context builder 不得把：

```text
null
missing
unsupported
data_insufficient
stale
```

轉成：

```text
0
```

或假裝資料存在。

AI report 必須能區分：

```text
No data
Zero
Unsupported
Not yet available
Stale
Partial
```

尤其財報與 Point-in-Time 資料不可因缺失而自行推論。

---

# 12. Point-in-Time Is Not mystocktracer's Job to Re-Invent

若 TWstock 提供：

```text
published_at
available_at
revision
revision_identity
```

mystocktracer 必須尊重它。

不得重新根據：

- period_end
- HTTP fetched time
- Last-Modified
- 法定申報期限
- 今天看到的最新版數值

自行判斷歷史 availability。

原則：

```text
query_at > available_at
```

由 canonical data layer 定義。

mystocktracer 只負責消費。

---

# 13. Provenance Must Survive Product Layers

mystocktracer 在顯示資料時，可以簡化 UI，但資料模型不得丟失必要來源資訊。

應保留能力表示：

```text
provider
source
source_url
retrieved_at
trade_date
period_end
published_at
available_at
status
revision
```

尤其當：

- fallback 被使用；
- official source unavailable；
- stale cache 被使用；
- third-party fallback；
- data partial；
- data insufficient

時，產品層必須有辦法知道。

不要在 UI adapter 裡把 provenance 丟掉。

---

# 14. Shared Data Consumer Architecture

mystocktracer 應逐步建立專門的 shared Taiwan data consumer。

建議概念：

```text
backend/internal/providers/taiwan/
    shared_dataset.go
```

或其他符合現有架構的命名。

此 adapter 的責任：

- 找到 shared Taiwan dataset；
- 驗證 schema version；
- 讀取 manifest；
- 判斷 domain availability；
- 判斷 freshness；
- 轉成 mystocktracer internal model；
- 在 unavailable 時 fallback；
- 不把 shared storage schema 滲透到整個 app。

禁止讓整個 codebase 到處直接讀：

```text
D:\TWMarketData\...
```

應只有一個 integration boundary。

---

# 15. External Shared Data Location

共享資料位置不得寫死。

應允許設定，例如：

```text
TAIWAN_DATA_DIR
```

概念：

```text
TaiwanData/
    manifest.json
    securities/
    daily/
    institutional/
    margin/
    fundamentals/
    valuation/
    dividends/
    cashflow/
    factors/
    research/
```

mystocktracer 必須：

- configuration-driven；
- Windows path-safe；
- 不依賴 TWstock repo 必須位於特定位置；
- 不直接 import TWstock source code。

---

# 16. Manifest Must Be Respected

禁止用單一：

```text
latest_date
```

判斷整個 Taiwan dataset 都是最新。

必須按 domain 判斷。

例如：

```json
{
  "daily": {
    "status": "available",
    "as_of": "..."
  },
  "institutional": {
    "status": "available",
    "as_of": "..."
  },
  "fundamentals": {
    "status": "partial"
  },
  "cashflow": {
    "status": "available"
  }
}
```

mystocktracer UI 必須能接受：

> 某些 domain 有資料，某些 domain 暫時沒有。

不得因此把整支股票標成「查詢失敗」。

---

# 17. Fallback Policy

mystocktracer 的既有 Taiwan provider 可以作為 fallback。

但 fallback 行為必須明確。

例如：

```text
Shared data available
→ shared

Shared data unavailable
→ official provider

Official provider unavailable
→ approved cached / fallback path

All unavailable
→ explicit error / unavailable
```

禁止：

```text
shared fail
→ silently return zero
```

禁止：

```text
official fail
→ silently use third party and pretend official
```

---

# 18. Product-Level Cache vs Market Dataset

mystocktracer 可以保留產品級快取。

例如：

- UI request cache
- stock detail cache
- AI result cache
- saved screener cache
- navigation cache
- per-session response cache

但不要再建另一套永久 Taiwan historical warehouse。

原則：

```text
Persistent canonical market history
        -> TWstock/shared layer

Short-lived product performance cache
        -> mystocktracer
```

不要混淆兩者。

---

# 19. Multi-Market Architecture

mystocktracer 未來可能不只有台股。

因此台灣共享資料 integration 不得讓整個產品架構變成 Taiwan-specific。

應保持：

```text
Product
   │
Market abstraction
   │
 ┌─┼───────────────┐
 │ │               │
TW US             future
 │
Taiwan shared provider
```

TWstock 是：

> Taiwan-specific canonical market engine

不是 mystocktracer 全產品 backend 的替代品。

---

# 20. Do Not Copy TWstock Source Into mystocktracer

禁止為了整合快速：

- copy Python parser 到 Go；
- copy normalization table；
- copy constants；
- copy PIT calculation；
- copy giant provider logic；
- copy fixture logic manually。

共享方式優先：

```text
contract
dataset
manifest
golden fixtures
API
adapter
```

不是 source duplication。

---

# 21. Cross-Language Contract

TWstock 與 mystocktracer 使用不同程式語言不是問題。

共同標準應為 language-neutral contract。

例如：

```text
taiwan-market-contract-v1.1
```

以及 golden fixtures。

兩專案應能針對相同 fixture 驗證：

- dates
- symbols
- market
- units
- null
- zero
- malformed input
- institutional quantities
- margin lot conversion
- availability
- revision semantics
- provenance

若 Python 與 Go 結果不同：

> 不允許各自認定自己正確。

必須回到 contract 判定。

---

# 22. Product Errors Must Be User-Safe

mystocktracer 是 user-facing product。

底層資料錯誤不能直接變成：

- panic；
- blank page；
- fake zero；
- fake successful response；
- misleading AI report。

應區分：

```text
loading
available
partial
stale
unsupported
data_insufficient
provider_error
```

產品層可以簡化 wording，但內部狀態必須保持明確。

---

# 23. Incremental Migration Only

目前兩專案都仍在開發。

禁止一次性「重構所有 Taiwan providers」。

採用：

> incremental migration

建議順序：

```text
Phase 1
Security Master

Phase 2
Daily Market Data

Phase 3
Institutional + Margin

Phase 4
Fundamentals + Valuation

Phase 5
Factors

Phase 6
Research Facts
```

每完成一個 domain：

1. shared adapter；
2. tests；
3. fallback；
4. migration；
5. verification；
6. 才進下一個。

---

# 24. Do Not Block Current Development Unnecessarily

Anti-duplication 不代表每次都要大重構。

如果目前任務只是：

- UI；
- layout；
- filter；
- watchlist；
- portfolio；
- alert；
- AI report rendering；
- market navigation；

而不涉及底層 market truth：

> 可以直接開發。

只有跨入 market-data ownership 時才要求完整跨專案 investigation。

---

# 25. Mandatory Investigation Before Market-Layer Changes

只要任務涉及台股底層資料，AI Agent 必須先調查。

至少：

```text
1. Search mystocktracer existing implementation.
2. Search TWstock existing implementation.
3. Identify canonical owner.
4. Identify existing schema.
5. Identify existing tests.
6. Identify current persistence/cache.
7. Identify provenance semantics.
8. Determine shared-data or adapter option.
9. Propose minimal implementation.
10. Only then modify code.
```

在 implementation plan 中必須寫：

```text
Domain:
Canonical owner:
Existing mystocktracer implementation:
Existing TWstock implementation:
Reusable capability:
Fallback requirement:
Required adapter:
Duplicate code avoided:
```

---

# 26. Preserve Working Code

禁止：

- 因為新 shared layer 就一次刪除全部 provider；
- 為了「架構一致」重寫可用功能；
- 無測試搬動大量 Go modules；
- 同時改 API、storage、schema、UI、provider；
- 無 rollback plan 的 migration；
- 無 golden fixture 的 normalization rewrite。

原則：

> Reuse first. Adapt second. Replace only after verification.

---

# 27. Tests Are Required

Shared Taiwan integration 至少考慮：

- adapter tests
- manifest parsing tests
- schema-version tests
- fallback tests
- freshness tests
- partial-domain tests
- stale-data tests
- null-vs-zero tests
- provenance tests
- cross-language golden fixture tests
- regression tests

不得只測：

```text
request returned HTTP 200
```

還要驗證：

```text
source
status
unit
date
freshness
revision
availability
fallback
```

---

# 28. Stop Condition

如果新需求會造成：

- 第二套 TWSE 主 provider；
- 第二套 TPEx 主 provider；
- 第二套 MOPS normalization；
- 第二套 historical warehouse；
- 第二套 PIT engine；
- 第二套 technical factor；
- 第二套 fundamental factor；
- 第二套 Research Facts；
- 第二套 identical Taiwan dataset cache；
- 第二套相同大型 archive download；

AI Agent 必須停止直接實作。

先提出：

```text
reuse
adapter
shared dataset
contract
fallback
```

方案。

只有在確認 canonical owner 沒有此能力，而且需求確實應屬 mystocktracer 後，才建立新實作。

---

# 29. New Feature Decision Tree

每個新功能先問：

## A. Is this Taiwan market truth?

例如：

- 新 MOPS 欄位
- 新資料來源
- 借券
- 董監持股
- ETF constituents
- 當沖統計
- 新財務 factor
- 新 technical factor

→ 預設由 TWstock/shared layer 負責。

mystocktracer 做 consumer / UI。

---

## B. Is this product behavior?

例如：

- 收藏
- Watchlist
- Portfolio
- 篩選器
- Saved screen
- Compare
- Alerts
- UI
- notification
- research history

→ mystocktracer 負責。

---

## C. Is this AI?

Market facts：

→ shared / TWstock

Prompt：

→ mystocktracer

Report：

→ mystocktracer

UI：

→ mystocktracer

---

## D. Is this deterministic product-independent calculation?

例如：

```text
revenue_yoy
foreign_net_5d
RS20
relative_volume
```

→ 優先 canonical shared factor。

---

## E. Is this subjective ranking?

例如：

```text
Hot Stock Score
AI Bullish Score
Watchlist Priority
```

→ mystocktracer 可自行擁有。

---

# 30. Long-Term Architecture

目標架構：

```text
                  Taiwan Market Core
                         │
          ┌──────────────┼──────────────┐
          │              │              │
       Market Data     Factors      Research Facts
          │              │              │
          └──────────────┼──────────────┘
                         │
                         ▼
                  mystocktracer
                         │
       ┌─────────────────┼─────────────────┐
       │                 │                 │
    Dashboard         Screener         AI Research
       │                 │                 │
    Watchlist          Radar             Reports
       │
    Portfolio
```

mystocktracer 不需要知道每份官方資料最底層如何下載與重建。

它需要知道：

```text
What data is available?
How fresh is it?
Where did it come from?
Can I safely display it?
How should users interact with it?
```

---

# 31. Final Development Principle

所有後續 mystocktracer 開發遵循：

> **mystocktracer owns the product, not a second Taiwan data warehouse.**

> **TWstock owns Taiwan market truth.**

> **Reuse market facts, build better product experiences.**

> **Existing working code is an asset, not something to rewrite automatically.**

> **Shared facts, separate opinions.**

> **One market fact, one canonical implementation.**

> **Fallback is allowed; duplicate primary ownership is not.**

若任務不確定屬於哪一個專案：

先判定 ownership，再實作。

如果另一專案已經完成而且能正常使用：

優先共用。

不要重新造輪子。