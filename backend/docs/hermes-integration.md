# AI runtime 與 Hermes adapter

mystocktracer 的後端產品契約位於 `backend/internal/agent`。`hermesadapter` 將該契約接到封裝的 Hermes Agent；前端對話只見 `/api/v1/ai/ws` 的版本化事件，不直接讀寫 Hermes JSON-RPC。模型連線測試與使用者明確啟動的台股 AI 研究也走同一產品邊界。詳細事件及隔離要求見 [AI runtime 契約](../../docs/ai-runtime-contract.md)。

桌面啟動時會先確定使用者資料目錄，再啟動 backend。canonical 環境變數使用 `MYSTOCKTRACER_HERMES_RUNTIME_ROOT`、`MYSTOCKTRACER_HERMES_HOME`、`MYSTOCKTRACER_HERMES_WORKDIR` 與 `MYSTOCKTRACER_SETTINGS_PATH`；`A_STOCK_*` 只保留對舊環境的讀取相容。安裝包提供獨立 Python/Hermes runtime，個人設定與工作目錄留在 userData。桌面身份與遷移先後順序見 [桌面資料遷移](../../docs/desktop-identity-migration.md)。

模型 provider、base URL、model、API mode 與回應逾時沿用既有設定 schema。一般 `settings.json` 不應保存模型密鑰；adapter 將 `MODEL_API_KEY` 及各 profile 密鑰保存於 Hermes Home。讀取 API 只顯示 configured/masked 狀態。清除金鑰時，子程序不可繼續使用宿主環境中的舊值。profile 切換保留各自密鑰及全域回應逾時，預設 300 秒、允許 30–3600 秒。

開發與封裝方式見 [開發文件](../../docs/development.md) 與 [桌面 runtime 契約](../../docs/desktop-runtime-contract.md)。第三方 Hermes Agent 固定版本與授權聲明見 [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md)。
