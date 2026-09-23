# OSS 來源稽核（B6，2026-09-23）

**結論：Technical OSS independence complete; no known code/provenance blocker remains, but LICENSE change requires explicit maintainer decision.**

本文件是來源候選與工程替換的紀錄，不是 clean-room 或法律權利證明。mystocktracer 仍由 jundizhou/easy-stock 衍生；原 LICENSE、署名、歷史 release notes 與 Git history 均保留。沒有在 B6 發布 release 或修改授權。

## 可重現方法

- Fork 比對點：b969d05984dda736de9ee3dc2a881e8c65a373c5。
- B6 前：已合併 B5 的 main，8282cd47fb3ef2b2b289a84aa539b8b108ffa67f。前後各使用當時的 SHA 綁定決策表，避免把 B6 證據倒灌到舊版本。
- B6 後：本分支目前 tree。執行 npm.cmd run audit:provenance -- --summary 取得摘要；省略 --summary 取得每檔來源候選、SHA、LOC 與舊名稱的行號。決策表見 [oss-provenance-decisions.json](oss-provenance-decisions.json)，行為與逐檔判定見 [B6 契約](oss-b6-contract.md)。
- 掃描納入所有 tracked 與未忽略的新檔案；以 CRLF 正規化 SHA、fork 同路徑／跨路徑 blob、Git rename/copy 及長語句重疊找候選。長語句門檻為至少四行、200 字元及目的檔有效字元的 20%。這些篩選不能單獨證明著作權或來源獨立。
- inherited LOC 是該類整個檔案的目前實體行數，含註解與空行，並非逐字存活的上游行數。code LOC 包含 Go、TS、JS、CSS、HTML、Python、Shell、PowerShell、JSON、YAML 等；法律與歷史文件不算 code LOC。
- SHA 不符的 decision 失效；未釐清的 artwork 與 stale decision 令 audit 指令失敗。已知法律與歷史文字列為 inherited，沒有因本次工程替換而重新標成 original。

## 同口徑結果

| 指標 | B6 前 | B6 後 |
| --- | ---: | ---: |
| 納入檔案 | 313 | 315 |
| inherited files | 67 | 14 |
| confirmed-inherited | 17 | 14 |
| likely-inherited | 50 | 0 |
| original（工程分類） | 243 | 298 |
| unclear provenance | 0 | 0 |
| third-party／標準依賴檔 | 3 | 3 |
| inherited LOC | 2,964 | 0 |
| 技術替換阻擋項 | 53 | 0 |
| 含法律／歷史的授權檢視項 | 67 | 14 |

剩餘 14 項恰為 LICENSE 一項與 .github/release-notes 的 13 份歷史文件。runtime、test、config、一般 docs、packaging tool 的 inherited／unclear 替換阻擋項均為零。授權檢視項保留 14，表示 LICENSE／歷史署名仍需維護者在未來法律與授權決策時檢視，不能將工程 gate 的零阻擋解讀為自動可改 LICENSE。

## B6 替換與判定

- 重寫 settings／Agent settings handler、設定合併與 view、通用 response helper；以原 API 的 JSON 鍵、路由、大小限制、狀態碼、secret 邊界與持久化相容為契約。
- 重寫 MarketDataViews 元件與 index 畫面 CSS；保留後端數值、日期、區間、來源、狀態、空／載入／錯誤語意，新增元件回歸案例。重寫 chat 與 runtime log helper。
- 重寫原 14 個 inherited test 路徑：backend 的 server/main、appsettings/store、HTTP AI/WebSocket、模型列表、持久化、server 與兩類 settings；frontend 的 Markdown、backend transport、chat、LLM provider 與 runtime log。使用合成 fixture，保留有效成功、失敗、邊界與秘密保護斷言。
- Issue／PR 模板、README、ROADMAP、CONTRIBUTING、SECURITY、四份 backend 文件及使用指南已改寫為 mystocktracer 的台股維護文字。
- backend agent/appsettings/foundation/HTTP contract DTO、frontend Vite/React 入口與 llm-providers 互通常數、Git／Go／TypeScript／npm 設定經逐檔檢視，屬 JSON／持久化／工具必需的功能性契約或標準宣告。沒有為了來源數字更動 schema、依賴或台股計算；每檔理由與測試連結在 [B6 契約](oss-b6-contract.md) 及 SHA 綁定決策表。
- easy-stock／a-stock-ai profile、A_STOCK_*／VITE_A_STOCK_* 與舊 NSIS GUID 只在讀取舊安裝、環境與資料時保留；canonical runtime 與發布來源使用 mystocktracer。來源掃描中的名稱出現不等同於活躍上游功能。

## Readiness gates

| Gate | B6 結果 | 證據與界線 |
| --- | --- | --- |
| A Implementation independence | PASS | 已知 substantive inherited runtime 為零；DTO／標準互通契約有逐檔決策及行為驗證。 |
| B Asset independence | PASS | B4 原創 generator 及六個 tracked 品牌素材仍在；unclear 0；Windows 成品實際載入素材通過。 |
| C Runtime / packaging independence | PASS（已驗證的 Windows 範圍） | 前後端建置、package preparation、Windows 成品 verifier、封裝串流及 EXE 遷移／storage smoke 通過；macOS 實機、正式安裝升級、簽章與公證本輪未測。 |
| D Test independence | PASS | 14 個 inherited test 路徑皆以行為測試替換；完整 Go、frontend、desktop、tooling 套件通過。 |
| E Legal / attribution | 保留完成 | LICENSE、上游署名、13 份歷史 release notes 與 Git history 未變更。 |
| F Relicensing | 需維護者明確決定 | 工程掃描沒有已知 code/provenance blocker；本報告不能授權 LICENSE 變更，目前授權照舊。 |

## B6 本機驗證

- backend：go test ./...、go vet ./...、go build ./... 通過。
- frontend：20 test files、833 tests 通過；TypeScript 與 Vite production build 通過。
- desktop：69 tests 通過，包含 migration、素材／package contract、update 與資料保護。
- tooling：22 tests 通過，含 provenance 分類回歸；audit stale decisions 為空；git diff --check 通過。
- Windows：package:windows 完成 preparation／electron-builder dir／成品 verifier；packaged fake-runtime streaming smoke 與實際 packaged desktop smoke 通過。未執行 release。
- GitHub CI 與 GitHub Codex Review 以本輪 PR 最終記錄為準；本機結果不代替兩者。
