# OSS 來源稽核（B6 與 MIT 授權決策，2026-09-23）

**結論：Technical OSS independence complete；維護者已明確決定現行 mystocktracer 採 MIT。歷史 upstream easy-stock 內容與第三方元件維持各自原有授權及 attribution。**

本文件是來源候選、工程替換與維護者授權決策的紀錄，不是 clean-room 或法律權利證明。mystocktracer 仍由 jundizhou/easy-stock 衍生；原 LICENSE 完整移存至 [歷史授權文件](../LICENSES/easy-stock-PolyForm-Noncommercial-1.0.0.txt)，署名、13 份歷史 release notes 與 Git history 均保留。B6 本身沒有發布 release 或修改授權；其後的本次 MIT 決策也不發布正式 release。

## 可重現方法

- Fork 比對點：b969d05984dda736de9ee3dc2a881e8c65a373c5。
- B6 前：已合併 B5 的 main，8282cd47fb3ef2b2b289a84aa539b8b108ffa67f。前後各使用當時的 SHA 綁定決策表，避免把 B6 證據倒灌到舊版本。
- B6 後／授權變更前：`f1dcacc6c2f2833106d2a37c1f95ebc01b880e1c`。本次沒有重做 B1–B6，也沒有變更產品實作。執行 npm.cmd run audit:provenance -- --summary 取得目前 tree 摘要；省略 --summary 取得每檔來源候選、SHA、LOC 與舊名稱的行號。決策表見 [oss-provenance-decisions.json](oss-provenance-decisions.json)，行為與逐檔判定見 [B6 契約](oss-b6-contract.md)。
- 掃描納入所有 tracked 與未忽略的新檔案；以 CRLF 正規化 SHA、fork 同路徑／跨路徑 blob、Git rename/copy 及長語句重疊找候選。長語句門檻為至少四行、200 字元及目的檔有效字元的 20%。這些篩選不能單獨證明著作權或來源獨立。
- inherited LOC 是該類整個檔案的目前實體行數，含註解與空行，並非逐字存活的上游行數。code LOC 包含 Go、TS、JS、CSS、HTML、Python、Shell、PowerShell、JSON、YAML 等；法律與歷史文件不算 code LOC。
- SHA 不符的 decision 失效；未釐清的 artwork 與 stale decision 令 audit 指令失敗。已知法律與歷史文字列為 inherited，沒有因本次工程替換而重新標成 original。

## 同口徑結果

| 指標 | B6 前 | B6 後／MIT 前 | MIT 變更後 |
| --- | ---: | ---: | ---: |
| 納入檔案 | 313 | 315 | 317 |
| inherited files | 67 | 14 | 14 |
| confirmed-inherited | 17 | 14 | 14 |
| likely-inherited | 50 | 0 | 0 |
| original（工程分類） | 243 | 298 | 300 |
| unclear provenance | 0 | 0 | 0 |
| third-party／標準依賴檔 | 3 | 3 | 3 |
| inherited code LOC | 2,964 | 0 | 0 |
| 技術替換阻擋項 | 53 | 0 | 0 |
| 含法律／歷史的授權檢視項 | 67 | 14 | 14 |

剩餘 14 項恰為移存的 `LICENSES/easy-stock-PolyForm-Noncommercial-1.0.0.txt` 與 `.github/release-notes` 的 13 份歷史文件（完整版本清單見 [NOTICE](../NOTICE.md)）。逐一比對 fork blob，CRLF 正規化 SHA 全數相符；本次未更動任何歷史 release note。原 PolyForm 文件的正規化 SHA-256 為 `c33d0f2551b1f6dd06d0643109c84ef8e8f4465dae0d3d2208d7ad60ff00963f`。

runtime、test、config、一般 docs、packaging tool 的 inherited／unclear 替換阻擋項均為零。`blockingFiles=14` 仍是對「把全部 repository 內容一律改授權」的檢視項，並非本次限定範圍 MIT 決策的技術 blocker；它們沒有被重新標成 original 或改採 MIT。歷史授權只有在上述精確路徑與 SHA 相符時才列入 legal area；仍以相同 blob／來源規則判定 inherited，不豁免任意 LICENSES 文字檔或被替換的授權檔。新增 NOTICE 按一般文件檢查，不新增路徑豁免。

## 維護者授權決策

2026-09-23 維護者明確決定 current mystocktracer 採 MIT。決策以 B6 契約、SHA 綁定證據及上述變更前 audit 為基礎：沒有已知 substantive implementation、inherited test 或 unclear provenance blocker。這是維護者的授權決策，不是掃描工具自動授予權利。

- 根目錄 [LICENSE](../LICENSE) 使用標準 [MIT 條款](https://opensource.org/license/mit)，涵蓋現行原創／重寫後的自有內容；採用標準授權文字不表示主張該條款文字為原創。
- [NOTICE](../NOTICE.md) 界定 MIT 自引入提交起適用的範圍、upstream 著作權及歷史例外；不追溯重新授權 easy-stock commits 或 release notes，也不將更早版本一律改套 PolyForm。
- 原 PolyForm 全文及 Required Notice 保留；[第三方通知](../THIRD_PARTY_NOTICES.md) 的 Hermes MIT 全文與版權保持不變，其他元件／外部資料仍依原有條款。
- root、frontend、desktop 的 package metadata 與 lockfile workspace metadata 統一為 MIT，沒有變更 dependency 版本。桌面封裝攜帶現行 LICENSE、NOTICE、歷史 PolyForm 與既有第三方通知，verifier 拒絕缺少授權文件的成品。
- 本次只更新已修改文件的 SHA 決策及新增 MIT LICENSE 決策；未更動的 B6 程式碼證據保持不變。

## B6 替換與判定

- 重寫 settings／Agent settings handler、設定合併與 view、通用 response helper；以原 API 的 JSON 鍵、路由、大小限制、狀態碼、secret 邊界與持久化相容為契約。
- 重寫 MarketDataViews 元件與 index 畫面 CSS；保留後端數值、日期、區間、來源、狀態、空／載入／錯誤語意，新增元件回歸案例。重寫 chat 與 runtime log helper。
- 重寫原 14 個 inherited test 路徑：backend 的 server/main、appsettings/store、HTTP AI/WebSocket、模型列表、持久化、server 與兩類 settings；frontend 的 Markdown、backend transport、chat、LLM provider 與 runtime log。使用合成 fixture，保留有效成功、失敗、邊界與秘密保護斷言。
- Issue／PR 模板、README、ROADMAP、CONTRIBUTING、SECURITY、四份 backend 文件及使用指南已改寫為 mystocktracer 的台股維護文字。
- backend agent/appsettings/foundation/HTTP contract DTO、frontend Vite/React 入口與 llm-providers 互通常數、Git／Go／TypeScript／npm 設定經逐檔檢視，屬 JSON／持久化／工具必需的功能性契約或標準宣告。沒有為了來源數字更動 schema、依賴或台股計算；每檔理由與測試連結在 [B6 契約](oss-b6-contract.md) 及 SHA 綁定決策表。
- easy-stock／a-stock-ai profile、A_STOCK_*／VITE_A_STOCK_* 與舊 NSIS GUID 只在讀取舊安裝、環境與資料時保留；canonical runtime 與發布來源使用 mystocktracer。來源掃描中的名稱出現不等同於活躍上游功能。

## Readiness gates

| Gate | 目前結果 | 證據與界線 |
| --- | --- | --- |
| A Implementation independence | PASS | 已知 substantive inherited runtime 為零；DTO／標準互通契約有逐檔決策及行為驗證。 |
| B Asset independence | PASS | B4 原創 generator 及六個 tracked 品牌素材仍在；unclear 0；Windows 成品實際載入素材通過。 |
| C Runtime / packaging independence | PASS（已驗證的 Windows 範圍） | 前後端建置、package preparation、Windows 成品 verifier、封裝串流及 EXE 遷移／storage smoke 通過；macOS 實機、正式安裝升級、簽章與公證本輪未測。 |
| D Test independence | PASS | 14 個 inherited test 路徑皆以行為測試替換；完整 Go、frontend、desktop、tooling 套件通過。 |
| E Legal / attribution | PASS（範圍已界定） | 原 LICENSE 全文移存；上游署名、13 份歷史 release notes 與 Git history 保留；第三方權利不變。 |
| F Relicensing | PASS（維護者已決定） | 現行自有程式碼採 MIT；歷史 upstream 與第三方內容明確排除於本次重新授權範圍。 |

## B6 本機驗證

- backend：go test ./...、go vet ./...、go build ./... 通過。
- frontend：20 test files、833 tests 通過；TypeScript 與 Vite production build 通過。
- desktop：69 tests 通過，包含 migration、素材／package contract、update 與資料保護。
- tooling：22 tests 通過，含 provenance 分類回歸；audit stale decisions 為空；git diff --check 通過。
- Windows：package:windows 完成 preparation／electron-builder dir／成品 verifier；packaged fake-runtime streaming smoke 與實際 packaged desktop smoke 通過。未執行 release。
- GitHub CI 與 GitHub Codex Review 以本輪 PR 最終記錄為準；本機結果不代替兩者。

## MIT 授權轉換驗證

- `npm.cmd run test:tooling`：23 tests 通過；包含歷史授權移存後仍為 inherited、仍需保留原權利，以及實作不因放進 LICENSES 目錄（含 `.py`、`.txt` 與冒用已知授權檔名）或改名 `NOTICE.md` 而豁免的回歸案例。精確路徑／SHA gate 與取消 NOTICE 豁免回應本次 Codex GitHub Review 的兩項 P2 findings。
- `npm.cmd --workspace desktop test`：70 tests 通過；含 Windows／macOS 缺少現行或歷史授權文件時拒絕成品的案例。
- 比對 B6 HEAD 與 fork：原 PolyForm 及 13 份 release notes 文字不變；Hermes 通知／MIT 全文不變；package 與 lockfile 除自有 license metadata 外沒有變更。
- 使用獨立 `desktop/dist/mit-license-resources` staging 完成 frontend／backend／Hermes preparation，以 `publish: never` 建立 `desktop/dist/mit-license-check/win-unpacked`；package verifier 通過。四份授權／通知檔與來源逐位元相符，`app.asar/package.json` 的 license 為 MIT。
- 對上述實際 Windows 成品執行 `packaged-desktop-smoke.mjs` 與 `packaged-fake-runtime-smoke.mjs`：身份、品牌、backend/frontend readiness、合成資料遷移、localStorage 關閉後的嚴格備份及封裝串流均通過。
- `npm.cmd run audit:provenance -- --summary`：如上表，technical blocker、inherited code LOC、unclear 均為 0，stale decisions 為空；`git diff --check` 通過。
- 本次未做 macOS 實機封裝、簽章、公證、正式 release 或 production deployment；GitHub CI 與 Codex GitHub Review 以本次授權轉換 PR 的最終記錄為準。
