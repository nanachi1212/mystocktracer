# OSS 來源稽核（2026-09-23）

**結論：OSS independence NOT complete。** B4 桌面身份與 B5 packaging/test cleanup 已完成本輪工程整理，但 substantive upstream implementation 與繼承測試仍存在；不可直接切換 LICENSE。原始 LICENSE、attribution、copyright 與 Git 歷史保留。

## 可重現基準與限制

- 稽核日期：2026-09-23；上游 fork：`b969d05984dda736de9ee3dc2a881e8c65a373c5`。
- B5 前基準為已合併 B4 / PR #12：`a20c872c28d5648655536aa8624a716393967506`。
- B5 後為包含本文件的提交 tree；執行 `npm.cmd run audit:provenance -- --summary` 重算摘要，省略 --summary 可取得逐檔 SHA、來源、LOC 與每個舊名稱出現的行號。
- 全部 Git tracked 與未忽略的新檔案均納入；node_modules、runtime downloads、dist、實際使用者資料等未追蹤產物不屬於 repository audit。沒有透過刪除有效測試、改名或排除 source 來降低數字。
- 比對 CRLF 正規化 SHA-256、同路徑 fork blobs、跨路徑相同內容、Git rename/copy（30%）與長語句重疊候選。拆檔掃描門檻為至少 4 個去除註解／import 的長語句、200 字元及目的檔有效字元的 20%。這是保守候選工具，無法偵測所有語意重寫或證明著作權。
- `confirmed-inherited` 是 fork 相同 bytes；`likely-inherited` 是來源候選仍缺完整替換證據。候選不能直接當法律上的可保護表達判定；但不能當作已獨立。
- original override 由 [決策表](oss-provenance-decisions.json) 綁定正規化 SHA 與 contract/phase 證據，不能僅因路徑相同而永久豁免。變更後的 stale decision 或無來源 artwork 令 audit command 失敗。已知 inherited 不使品質 CI 失敗，否則會掩蓋本報告誠實保留的 blocker。
- LOC 為被分類 inherited 的**整個目前檔案實體行數**，不是逐字存活的上游行數。含空白／註解，擴充至 Go/TS/JS/CSS/HTML、Python、Shell、PowerShell、JSON、YAML。二進位與文件仍逐檔盤點，但不算 code LOC。
- dependency lockfiles 與標準 Vite type directive 分列第三方，不因上游曾使用同套件便稱其 inherited easy-stock implementation。普通 package metadata 尚待逐項判定，見下表。
- 本報告是工程來源篩查與替換記錄，**不是 clean-room 或法律權利證明**。

## 同口徑前後結果

| 指標 | B4 合併後／B5 前 | B5 後 |
| --- | ---: | ---: |
| 全部納入檔案 | 306 | 313 |
| inherited files | 80 | 67 |
| confirmed-inherited | 21 | 17 |
| likely-inherited | 59 | 50 |
| original（工程分類） | 223 | 243 |
| unclear provenance | 0 | 0 |
| 第三方解析／標準檔 | 3 | 3 |
| inherited LOC（擴充口徑） | 3882 | 2964 |

舊工具在最終 B4 tree 為 97 files / 3,337 LOC / unclear 0；最初 B4 提交為 97 / 3,310。舊工具只以同路徑與較窄副檔名計算，漏掉拆出的表達，也沒有納入 B4 替換證據。新舊口徑不可當作單純刪碼減量。以上 80 → 67、3,882 → 2,964 才是同一新版方法套在 B4 與 B5 的比較。

| 剩餘類型 | 檔案數 | code LOC |
| --- | ---: | ---: |
| runtime | 19 | 1128 |
| test | 14 | 1539 |
| config | 10 | 297 |
| document | 10 | 0 |
| packaging-tool | 0 | 0 |
| historical-document | 13 | 0 |
| legal | 1 | 0 |

## 本輪替換與保留

B4 原創桌面 lifecycle、backend executable、preload bridge、更新來源、migration、品牌 generator 與打包 helpers 已在 PR #12 合併。GitHub Review 發現的外部引用開啟、日誌遮罩、Windows 活躍 storage 鎖、DB-only profile、清理失敗退出與失敗備份 staging 都有實際修正及回歸證據。

B5 依 [cleanup 契約](oss-cleanup-contract.md) 重寫以下 13 個原 inherited 路徑；保留原行為並加強負向測試，沒有靠移動路徑改分類：

- `desktop/test/hermes-runtime-root.test.cjs`
- `desktop/test/hermes-runtime.test.cjs`
- `desktop/test/runtime-logger.test.cjs`
- `desktop/test/backend-process.test.cjs`
- `desktop/test/data-protection.test.cjs`
- `desktop/test/update-manager.test.cjs`
- `desktop/test/updater-preload.test.cjs`
- `frontend/src/components/AppUpdatePanel.test.ts`
- `backend/internal/runtimelog/writer_test.go`
- `scripts/rebuild-restart.sh`
- `docs/development.md`
- `.github/workflows/release.yml`
- `desktop/AUTO_UPDATE.md`

另新增共用合成 fixture、跨平台開發啟動器與 provenance 回歸測試；限制 package staging 只能指向 desktop/dist 專用子目錄；canonical 環境變數向子程序傳遞，舊名稱保留讀取 fallback。發布 workflow 重新建立三平台 matrix，指定 tag 在每個 job checkout，沿用既有簽章介面與 SHA 驗證；沒有觸發發布。

原創工程區域包含既有台股 adapters/產品 UI、B2 runtime boundary、B3 agent adapter、B4 desktop/migration/brand 及本輪 tooling；其中下列拆檔候選**排除於整體原創聲明之外**。產品方向已明確轉為 maintenance，新的台股核心功能放在 TWstockfor_tick-stock-panel；本輪未變更 market truth、PIT 或 provider。

## 拆檔追蹤：先前被新路徑漏算的候選

對下列檔案逐項查看後，可見既有 DTO/schema 或 handler/response 表達延續；不是只因重新命名便判為原創。保留資料與 API 契約，不為清零而更動 Taiwan 計算／序列化。

| 現存檔案 | 上游來源候選 |
| --- | --- |
| `backend/internal/agent/types.go` | `backend/internal/hermes/runtime.go` |
| `backend/internal/appsettings/model.go` | `backend/internal/appsettings/store.go` |
| `backend/internal/foundation/market_data.go` | `backend/internal/foundation/types.go` |
| `backend/internal/foundation/market_index.go` | `backend/internal/foundation/market_overview.go` |
| `backend/internal/foundation/source.go` | `backend/internal/foundation/types.go` |
| `backend/internal/httpapi/agent_settings_contract.go` | `backend/internal/httpapi/settings_agent.go` |
| `backend/internal/httpapi/agent_settings_handler.go` | `backend/internal/httpapi/settings_agent.go` |
| `backend/internal/httpapi/agent_settings_model.go` | `backend/internal/httpapi/settings_agent.go` |
| `backend/internal/httpapi/response.go` | `backend/internal/httpapi/server.go` |
| `backend/internal/httpapi/settings_contract.go` | `backend/internal/httpapi/settings.go` |
| `backend/internal/httpapi/settings_handler.go` | `backend/internal/httpapi/settings.go` |
| `backend/internal/httpapi/settings_view.go` | `backend/internal/httpapi/settings.go` |

## 素材與第三方

所有 6 個 tracked artwork 都來自原創幾何 generator。執行 `node desktop/scripts/generate-brand.mjs` 後，`git diff --exit-code -- desktop/assets frontend/public` 通過。沒有 tracked font、sample screenshot 或剩餘 easy-stock artwork；UI 資產引用與 exact packaged 圖片載入亦已驗證。

| 素材 | 分類 | SHA-256 |
| --- | --- | --- |
| `desktop/assets/mystocktracer-mark.svg` | original | `d60f789f563f665c4732125584b8af9ab07ad68f63ffa405ec319cc7ff12201b` |
| `desktop/assets/mystocktracer.icns` | original | `687879b1256cfe9cd874cecda9e32918dd219ac247e7b0be8ae09a92abf02f58` |
| `desktop/assets/mystocktracer.ico` | original | `4f86320d4669531136cf95a594e161d24e2e5e436a7c11405e35727494a68578` |
| `desktop/assets/mystocktracer.png` | original | `a11d32a5965a98f57c27812b2a05a3ea628ceccbea47e9d9ccc16ae5bfa7091b` |
| `frontend/public/mystocktracer-mark.png` | original | `c66f417e038d3eacf4e7ccb79c5935ca087bc0362c2efed917027dcff8525811` |
| `frontend/public/mystocktracer-mark.svg` | original | `d60f789f563f665c4732125584b8af9ab07ad68f63ffa405ec319cc7ff12201b` |

| 第三方 | 本機鎖定／封裝版本 | 個別授權依據 |
| --- | --- | --- |
| Electron | 44.2.0 | package metadata / MIT |
| electron-builder | 26.15.3 | package metadata / MIT |
| electron-updater | 6.8.9 | package metadata / MIT |
| agent-browser | 0.25.3 | package metadata / Apache-2.0 |
| React | 19.2.7 | package metadata / MIT |
| Vite | 7.3.6 | package metadata / MIT |
| lucide-react | 0.545.0 | package metadata / ISC |
| react-markdown | 10.1.0 | package metadata / MIT |
| Hermes Agent | 0.18.2 | 封裝 LICENSE / MIT，保留 NousResearch copyright |

這些依賴正常保留，清單不是完整 SBOM，也不代替各 dependency 的授權義務。完整解析由 package-lock.json、backend/go.mod/go.sum、runtime-manifest.json 記錄，發布保留 [THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md) 與 Hermes LICENSE。本輪不升級 dependency。

## 所有剩餘 inherited 檔案

「是」表示不能在尚未替換／確認權利前宣稱該檔已可重新授權；不等同每行均屬可保護表達。legal/history 項目保留且不是 runtime replacement 工作，需在正式 LICENSE 決策時界定範圍。retained-contract-compatibility 的 DTO 不是可以忽略 provenance 的理由。

| 路徑 | 類型／分類 | fork 來源 | LOC | 保留原因 | 替換必要性／relicensing blocker |
| --- | --- | --- | ---: | --- | --- |
| `.github/ISSUE_TEMPLATE/bug_report.yml` | config / likely | `.github/ISSUE_TEMPLATE/bug_report.yml` | 81 | 同路徑設定／metadata／模板，尚未逐項證明標準宣告與原始表達邊界 | 替換／權利審核；是 |
| `.github/ISSUE_TEMPLATE/config.yml` | config / likely | `.github/ISSUE_TEMPLATE/config.yml` | 14 | 同路徑設定／metadata／模板，尚未逐項證明標準宣告與原始表達邊界 | 替換／權利審核；是 |
| `.github/ISSUE_TEMPLATE/feature_request.yml` | config / likely | `.github/ISSUE_TEMPLATE/feature_request.yml` | 85 | 同路徑設定／metadata／模板，尚未逐項證明標準宣告與原始表達邊界 | 替換／權利審核；是 |
| `.github/PULL_REQUEST_TEMPLATE.md` | document / likely | `.github/PULL_REQUEST_TEMPLATE.md` | 0 | 文件雖經台股化更新，尚無完整原創 replacement 證據 | 替換／權利審核；是 |
| `.github/release-notes/v0.1.0.md` | historical-document / confirmed | `.github/release-notes/v0.1.0.md` | 0 | 歷史發布文字／署名；不刪歷史以追求零數字 | 保留；法律範圍待決，非 runtime blocker |
| `.github/release-notes/v0.2.0.md` | historical-document / confirmed | `.github/release-notes/v0.2.0.md` | 0 | 歷史發布文字／署名；不刪歷史以追求零數字 | 保留；法律範圍待決，非 runtime blocker |
| `.github/release-notes/v0.3.0.md` | historical-document / confirmed | `.github/release-notes/v0.3.0.md` | 0 | 歷史發布文字／署名；不刪歷史以追求零數字 | 保留；法律範圍待決，非 runtime blocker |
| `.github/release-notes/v0.4.0.md` | historical-document / confirmed | `.github/release-notes/v0.4.0.md` | 0 | 歷史發布文字／署名；不刪歷史以追求零數字 | 保留；法律範圍待決，非 runtime blocker |
| `.github/release-notes/v0.5.0.md` | historical-document / confirmed | `.github/release-notes/v0.5.0.md` | 0 | 歷史發布文字／署名；不刪歷史以追求零數字 | 保留；法律範圍待決，非 runtime blocker |
| `.github/release-notes/v0.5.1.md` | historical-document / confirmed | `.github/release-notes/v0.5.1.md` | 0 | 歷史發布文字／署名；不刪歷史以追求零數字 | 保留；法律範圍待決，非 runtime blocker |
| `.github/release-notes/v0.6.0.md` | historical-document / confirmed | `.github/release-notes/v0.6.0.md` | 0 | 歷史發布文字／署名；不刪歷史以追求零數字 | 保留；法律範圍待決，非 runtime blocker |
| `.github/release-notes/v0.6.1.md` | historical-document / confirmed | `.github/release-notes/v0.6.1.md` | 0 | 歷史發布文字／署名；不刪歷史以追求零數字 | 保留；法律範圍待決，非 runtime blocker |
| `.github/release-notes/v0.7.0.md` | historical-document / confirmed | `.github/release-notes/v0.7.0.md` | 0 | 歷史發布文字／署名；不刪歷史以追求零數字 | 保留；法律範圍待決，非 runtime blocker |
| `.github/release-notes/v0.8.0.md` | historical-document / confirmed | `.github/release-notes/v0.8.0.md` | 0 | 歷史發布文字／署名；不刪歷史以追求零數字 | 保留；法律範圍待決，非 runtime blocker |
| `.github/release-notes/v0.9.0.md` | historical-document / confirmed | `.github/release-notes/v0.9.0.md` | 0 | 歷史發布文字／署名；不刪歷史以追求零數字 | 保留；法律範圍待決，非 runtime blocker |
| `.github/release-notes/v0.9.1.md` | historical-document / confirmed | `.github/release-notes/v0.9.1.md` | 0 | 歷史發布文字／署名；不刪歷史以追求零數字 | 保留；法律範圍待決，非 runtime blocker |
| `.github/release-notes/v0.9.2.md` | historical-document / confirmed | `.github/release-notes/v0.9.2.md` | 0 | 歷史發布文字／署名；不刪歷史以追求零數字 | 保留；法律範圍待決，非 runtime blocker |
| `.gitignore` | config / likely | `.gitignore` | 0 | 同路徑設定／metadata／模板，尚未逐項證明標準宣告與原始表達邊界 | 替換／權利審核；是 |
| `CONTRIBUTING.md` | document / likely | `CONTRIBUTING.md` | 0 | 文件雖經台股化更新，尚無完整原創 replacement 證據 | 替換／權利審核；是 |
| `LICENSE` | legal / confirmed | `LICENSE` | 0 | 現行授權必須保留；變更需明確權利決策 | 保留；法律範圍待決，非 runtime blocker |
| `README.md` | document / likely | `README.md` | 0 | 文件雖經台股化更新，尚無完整原創 replacement 證據 | 替換／權利審核；是 |
| `ROADMAP.md` | document / likely | `ROADMAP.md` | 0 | 文件雖經台股化更新，尚無完整原創 replacement 證據 | 替換／權利審核；是 |
| `SECURITY.md` | document / likely | `SECURITY.md` | 0 | 文件雖經台股化更新，尚無完整原創 replacement 證據 | 替換／權利審核；是 |
| `backend/cmd/server/main_test.go` | test / likely | `backend/cmd/server/main_test.go` | 98 | 有效回歸測試仍有上游來源；需以行為契約替換並保留斷言 | 替換／權利審核；是 |
| `backend/docs/api-routes.md` | document / likely | `backend/docs/api-routes.md` | 0 | 文件雖經台股化更新，尚無完整原創 replacement 證據 | 替換／權利審核；是 |
| `backend/docs/data-sources.md` | document / likely | `backend/docs/data-sources.md` | 0 | 文件雖經台股化更新，尚無完整原創 replacement 證據 | 替換／權利審核；是 |
| `backend/docs/hermes-integration.md` | document / likely | `backend/docs/hermes-integration.md` | 0 | 文件雖經台股化更新，尚無完整原創 replacement 證據 | 替換／權利審核；是 |
| `backend/docs/live-tests.md` | document / confirmed | `backend/docs/live-tests.md` | 0 | 文件雖經台股化更新，尚無完整原創 replacement 證據 | 替換／權利審核；是 |
| `backend/go.mod` | config / likely | `backend/go.mod` | 0 | 同路徑設定／metadata／模板，尚未逐項證明標準宣告與原始表達邊界 | 替換／權利審核；是 |
| `backend/internal/agent/types.go` | runtime / likely | `backend/internal/hermes/runtime.go` | 88 | 既有資料／JSON／agent 契約；保留 schema 以免破壞資料或 PIT，需釐清原始表達 | 替換／權利審核；是 |
| `backend/internal/appsettings/model.go` | runtime / likely | `backend/internal/appsettings/store.go` | 55 | 既有資料／JSON／agent 契約；保留 schema 以免破壞資料或 PIT，需釐清原始表達 | 替換／權利審核；是 |
| `backend/internal/appsettings/store_test.go` | test / likely | `backend/internal/appsettings/store_test.go` | 194 | 有效回歸測試仍有上游來源；需以行為契約替換並保留斷言 | 替換／權利審核；是 |
| `backend/internal/foundation/market_data.go` | runtime / likely | `backend/internal/foundation/types.go` | 32 | 既有資料／JSON／agent 契約；保留 schema 以免破壞資料或 PIT，需釐清原始表達 | 替換／權利審核；是 |
| `backend/internal/foundation/market_index.go` | runtime / likely | `backend/internal/foundation/market_overview.go` | 29 | 既有資料／JSON／agent 契約；保留 schema 以免破壞資料或 PIT，需釐清原始表達 | 替換／權利審核；是 |
| `backend/internal/foundation/source.go` | runtime / likely | `backend/internal/foundation/types.go` | 22 | 既有資料／JSON／agent 契約；保留 schema 以免破壞資料或 PIT，需釐清原始表達 | 替換／權利審核；是 |
| `backend/internal/httpapi/agent_settings_contract.go` | runtime / likely | `backend/internal/httpapi/settings_agent.go` | 51 | 既有資料／JSON／agent 契約；保留 schema 以免破壞資料或 PIT，需釐清原始表達 | 替換／權利審核；是 |
| `backend/internal/httpapi/agent_settings_handler.go` | runtime / likely | `backend/internal/httpapi/settings_agent.go` | 66 | 仍有來源關係或未完成替換證據；需依契約逐項重寫或取得權利 | 替換／權利審核；是 |
| `backend/internal/httpapi/agent_settings_model.go` | runtime / likely | `backend/internal/httpapi/settings_agent.go` | 100 | 既有資料／JSON／agent 契約；保留 schema 以免破壞資料或 PIT，需釐清原始表達 | 替換／權利審核；是 |
| `backend/internal/httpapi/ai_chat_test.go` | test / likely | `backend/internal/httpapi/ai_chat_test.go` | 64 | 有效回歸測試仍有上游來源；需以行為契約替換並保留斷言 | 替換／權利審核；是 |
| `backend/internal/httpapi/hermes_test.go` | test / likely | `backend/internal/httpapi/hermes_test.go` | 137 | 有效回歸測試仍有上游來源；需以行為契約替換並保留斷言 | 替換／權利審核；是 |
| `backend/internal/httpapi/llm_models_test.go` | test / likely | `backend/internal/httpapi/llm_models_test.go` | 138 | 有效回歸測試仍有上游來源；需以行為契約替換並保留斷言 | 替換／權利審核；是 |
| `backend/internal/httpapi/persistence_test.go` | test / likely | `backend/internal/httpapi/persistence_test.go` | 54 | 有效回歸測試仍有上游來源；需以行為契約替換並保留斷言 | 替換／權利審核；是 |
| `backend/internal/httpapi/response.go` | runtime / likely | `backend/internal/httpapi/server.go` | 81 | 仍有來源關係或未完成替換證據；需依契約逐項重寫或取得權利 | 替換／權利審核；是 |
| `backend/internal/httpapi/server_test.go` | test / likely | `backend/internal/httpapi/server_test.go` | 143 | 有效回歸測試仍有上游來源；需以行為契約替換並保留斷言 | 替換／權利審核；是 |
| `backend/internal/httpapi/settings_agent_test.go` | test / likely | `backend/internal/httpapi/settings_agent_test.go` | 122 | 有效回歸測試仍有上游來源；需以行為契約替換並保留斷言 | 替換／權利審核；是 |
| `backend/internal/httpapi/settings_contract.go` | runtime / likely | `backend/internal/httpapi/settings.go` | 100 | 既有資料／JSON／agent 契約；保留 schema 以免破壞資料或 PIT，需釐清原始表達 | 替換／權利審核；是 |
| `backend/internal/httpapi/settings_handler.go` | runtime / likely | `backend/internal/httpapi/settings.go` | 188 | 仍有來源關係或未完成替換證據；需依契約逐項重寫或取得權利 | 替換／權利審核；是 |
| `backend/internal/httpapi/settings_test.go` | test / likely | `backend/internal/httpapi/settings_test.go` | 325 | 有效回歸測試仍有上游來源；需以行為契約替換並保留斷言 | 替換／權利審核；是 |
| `backend/internal/httpapi/settings_view.go` | runtime / likely | `backend/internal/httpapi/settings.go` | 54 | 既有資料／JSON／agent 契約；保留 schema 以免破壞資料或 PIT，需釐清原始表達 | 替換／權利審核；是 |
| `desktop/package.json` | config / likely | `desktop/package.json` | 29 | 同路徑設定／metadata／模板，尚未逐項證明標準宣告與原始表達邊界 | 替換／權利審核；是 |
| `docs/user-guide.md` | document / likely | `docs/user-guide.md` | 0 | 文件雖經台股化更新，尚無完整原創 replacement 證據 | 替換／權利審核；是 |
| `frontend/index.html` | runtime / likely | `frontend/index.html` | 16 | 仍有來源關係或未完成替換證據；需依契約逐項重寫或取得權利 | 替換／權利審核；是 |
| `frontend/package.json` | config / likely | `frontend/package.json` | 29 | 同路徑設定／metadata／模板，尚未逐項證明標準宣告與原始表達邊界 | 替換／權利審核；是 |
| `frontend/src/components/MarkdownContent.test.tsx` | test / likely | `frontend/src/components/MarkdownContent.test.tsx` | 56 | 有效回歸測試仍有上游來源；需以行為契約替換並保留斷言 | 替換／權利審核；是 |
| `frontend/src/components/market/MarketDataViews.tsx` | runtime / likely | `frontend/src/components/market/MarketDataViews.tsx` | 108 | 仍有上游市場表格與數值呈現表達；本輪不變更產品計算 | 替換／權利審核；是 |
| `frontend/src/global.d.ts` | config / likely | `frontend/src/global.d.ts` | 9 | 同路徑設定／metadata／模板，尚未逐項證明標準宣告與原始表達邊界 | 替換／權利審核；是 |
| `frontend/src/lib/backend.test.ts` | test / likely | `frontend/src/lib/backend.test.ts` | 115 | 有效回歸測試仍有上游來源；需以行為契約替換並保留斷言 | 替換／權利審核；是 |
| `frontend/src/lib/chat.test.ts` | test / likely | `frontend/src/lib/chat.test.ts` | 39 | 有效回歸測試仍有上游來源；需以行為契約替換並保留斷言 | 替換／權利審核；是 |
| `frontend/src/lib/chat.ts` | runtime / likely | `frontend/src/lib/chat.ts` | 43 | 仍有來源關係或未完成替換證據；需依契約逐項重寫或取得權利 | 替換／權利審核；是 |
| `frontend/src/lib/llm-providers.test.ts` | test / likely | `frontend/src/lib/llm-providers.test.ts` | 39 | 有效回歸測試仍有上游來源；需以行為契約替換並保留斷言 | 替換／權利審核；是 |
| `frontend/src/lib/llm-providers.ts` | runtime / likely | `frontend/src/lib/llm-providers.ts` | 35 | 仍有來源關係或未完成替換證據；需依契約逐項重寫或取得權利 | 替換／權利審核；是 |
| `frontend/src/lib/runtime-log.test.ts` | test / likely | `frontend/src/lib/runtime-log.test.ts` | 15 | 有效回歸測試仍有上游來源；需以行為契約替換並保留斷言 | 替換／權利審核；是 |
| `frontend/src/lib/runtime-log.ts` | runtime / likely | `frontend/src/lib/runtime-log.ts` | 39 | 仍有來源關係或未完成替換證據；需依契約逐項重寫或取得權利 | 替換／權利審核；是 |
| `frontend/src/main.tsx` | runtime / likely | `frontend/src/main.tsx` | 10 | 仍有來源關係或未完成替換證據；需依契約逐項重寫或取得權利 | 替換／權利審核；是 |
| `frontend/tsconfig.json` | config / confirmed | `frontend/tsconfig.json` | 21 | 同路徑設定／metadata／模板，尚未逐項證明標準宣告與原始表達邊界 | 替換／權利審核；是 |
| `frontend/vite.config.ts` | runtime / confirmed | `frontend/vite.config.ts` | 11 | 仍有來源關係或未完成替換證據；需依契約逐項重寫或取得權利 | 替換／權利審核；是 |
| `package.json` | config / likely | `package.json` | 29 | 同路徑設定／metadata／模板，尚未逐項證明標準宣告與原始表達邊界 | 替換／權利審核；是 |

## 刻意保留的舊名稱

- attribution/legal：原作者、LICENSE、fork 關係、歷史 release notes；不刪除。
- backward compatibility/migration：easy-stock / desktop / a-stock-ai profile、A_STOCK_* / VITE_A_STOCK_*、舊 NSIS GUID、既有 live-test flags。保持來源資料與原安裝可恢復；目前 app/runtime/feed 使用 mystocktracer。
- original regression test：合成 legacy profile、環境優先序與拒絕舊橋接／資產的斷言；不是重新啟用 upstream 功能。
- inherited implementation：表中仍待替換的 source/tests/docs；不能把其中舊名稱全部說成 legal。
- third-party dependency：標準工具變數與授權文件屬個別第三方，不是以 easy-stock 為權利來源。

以下為每個出現舊名稱檔案的分類。自動分類按行語境初篩；精確行號與 SHA 見完整 audit JSON，法律與技術意義以本文逐檔表為準。

| 位置 | 出現類型 |
| --- | --- |
| `.github/release-notes/v0.1.0.md` | legal-attribution |
| `.github/release-notes/v0.2.0.md` | legal-attribution |
| `.github/release-notes/v0.3.0.md` | legal-attribution |
| `.github/release-notes/v0.4.0.md` | legal-attribution |
| `.github/release-notes/v0.5.0.md` | legal-attribution |
| `.github/release-notes/v0.5.1.md` | legal-attribution |
| `.github/release-notes/v0.6.0.md` | legal-attribution |
| `.github/release-notes/v0.6.1.md` | legal-attribution |
| `.github/release-notes/v0.7.0.md` | legal-attribution |
| `.github/release-notes/v0.8.0.md` | legal-attribution |
| `.github/release-notes/v0.9.0.md` | legal-attribution |
| `.github/release-notes/v0.9.1.md` | legal-attribution |
| `.github/release-notes/v0.9.2.md` | legal-attribution |
| `AGENTS.md` | legal-attribution |
| `CONTRIBUTING.md` | legal-attribution |
| `LICENSE` | legal-attribution |
| `README.md` | legal-attribution、inherited-implementation |
| `ROADMAP.md` | legal-attribution、inherited-implementation |
| `SECURITY.md` | legal-attribution |
| `backend/cmd/server/main_test.go` | inherited-implementation |
| `backend/cmd/server/profile_compatibility_test.go` | original-regression-test |
| `backend/cmd/server/runtime_config.go` | backward-compatibility-migration |
| `backend/docs/api-routes.md` | inherited-implementation |
| `backend/docs/hermes-integration.md` | inherited-implementation |
| `backend/docs/live-tests.md` | inherited-implementation |
| `backend/internal/providers/taiwan/live_test.go` | original-regression-test |
| `backend/internal/providers/toalpha/live_test.go` | original-regression-test |
| `backend/internal/taiwanportfolio/store.go` | backward-compatibility-migration |
| `backend/internal/taiwanwatchlist/store.go` | backward-compatibility-migration |
| `desktop/AUTO_UPDATE.md` | backward-compatibility-migration |
| `desktop/data-protection.cjs` | backward-compatibility-migration |
| `desktop/scripts/browser-bin/agent-browser` | backward-compatibility-migration |
| `desktop/scripts/build-config.mjs` | legal-attribution |
| `desktop/scripts/electron-builder.mjs` | backward-compatibility-migration |
| `desktop/scripts/hermes-runtime.mjs` | backward-compatibility-migration |
| `desktop/scripts/package-resources.mjs` | backward-compatibility-migration |
| `desktop/scripts/packaged-desktop-smoke.mjs` | backward-compatibility-migration |
| `desktop/scripts/prepare-package.mjs` | backward-compatibility-migration |
| `desktop/scripts/release-tools.mjs` | backward-compatibility-migration |
| `desktop/scripts/verify-release-package.mjs` | backward-compatibility-migration |
| `desktop/state-copy.py` | backward-compatibility-migration |
| `desktop/test/agent-browser-wrapper.test.cjs` | original-regression-test |
| `desktop/test/release-metadata.test.cjs` | original-regression-test |
| `desktop/test/update-feed.test.cjs` | original-regression-test |
| `desktop/test/user-data-migration.test.cjs` | original-regression-test |
| `desktop/update-feed.cjs` | legal-attribution |
| `desktop/user-data-migration.cjs` | backward-compatibility-migration |
| `docs/TOALPHA_MOPS_INTEGRATION_AUDIT.md` | backward-compatibility-migration |
| `docs/brand-assets.md` | backward-compatibility-migration |
| `docs/desktop-identity-migration.md` | backward-compatibility-migration |
| `docs/development.md` | legal-attribution、backward-compatibility-migration |
| `docs/oss-cleanup-contract.md` | legal-attribution |
| `docs/oss-provenance-audit.md` | legal-attribution |
| `docs/runtime-contract.md` | backward-compatibility-migration |
| `docs/taiwan-migration-audit.md` | backward-compatibility-migration |
| `docs/taiwan-roadmap.md` | backward-compatibility-migration |
| `frontend/src/components/AIChatWorkspace.tsx` | backward-compatibility-migration |
| `frontend/src/lib/backend.test.ts` | inherited-implementation |
| `frontend/src/lib/backend.ts` | backward-compatibility-migration |
| `scripts/dev.mjs` | backward-compatibility-migration |
| `scripts/dev.test.mjs` | original-regression-test |
| `scripts/provenance-audit.mjs` | backward-compatibility-migration |
| `scripts/rebuild-restart.test.mjs` | original-regression-test |

## Readiness gates

| Gate | 結果 | 證據與限制 |
| --- | --- | --- |
| A Implementation independence | FAIL | MarketDataViews、settings/response handlers 及其餘 source/DTO 仍有來源候選或實質上游表達，詳見逐檔表 |
| B Asset independence | PASS | 六個素材有可重現 generator 與內容 hash；unclear 0 |
| C Runtime / packaging independence | 功能獨立；來源 gate 部分完成 | canonical app/backend/feed/artifacts 已驗證；不再依賴上游服務。一般 package/config provenance 仍待釐清；macOS 實機與正式 installer upgrade/signing 沒有在本次 Windows 驗證 |
| D Test independence | FAIL | 本輪替換九個測試檔，但 backend HTTP/settings 及 frontend transport/helper 等繼承測試仍存在 |
| E Legal / attribution | 保留完成；僅剩法律／相容名稱的宣稱不成立 | LICENSE/attribution/history 完整保留；仍有 inherited source/tests/docs，不可全說成 legal-only |
| F Relicensing | NO | A/D 仍未過；程式 audit 也不能自行決定法律權利。LICENSE 變更需維護者／法律明確決定 |

**最小 remaining work**：先對逐檔表的 runtime handlers/renderer 與契約 DTO 界定原始表達及必要相容性，依既有 contract 替換或取得權利；再將剩餘有效測試改為獨立行為測試，逐項釐清 config/templates/docs（標準宣告可提供證據，不能機械重寫充數）。重算 audit、維持 full regression 與資料保護後，才交維護者作法律／授權範圍決策。這些都是 independence 維護工作，不開啟新產品功能階段。

## 驗證紀錄

B4 已完成 PR #12 的 CI、GitHub Codex Review 與 squash merge。B5 本機 Go build/vet/tests、frontend 840 tests + production build、desktop 69 tests（含 migration/asset/update）、tooling 21 tests 均通過。品牌重建無 tracked 內容差異；workflow YAML 已解析驗證。Windows package preparation（複用已驗證 Hermes runtime）、成品 verifier、實際 EXE migration/storage smoke 與兩段 streaming smoke 均通過；GitHub CI/Review 狀態以本輪 PR 的最終記錄為準，未執行的正式 release、macOS installer、Authenticode/notarization 不列為 PASS。
