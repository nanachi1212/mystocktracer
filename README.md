# mystocktracer

mystocktracer 是本機優先的台灣股票研究工作台。它整合 TWSE、TPEx 與 MOPS 等官方資料，提供每日總覽、自選股、持倉、公司事件提醒、個股證據與研究歷史。AI 研究和一般 AI 對話必須由使用者主動啟動；模型輸出不構成投資建議。

![mystocktracer 標誌](desktop/assets/mystocktracer-mark.svg)

## 使用

請從本 repository 的 [GitHub Releases](https://github.com/nanachi1212/mystocktracer/releases) 取得桌面程式，並先閱讀 [使用說明](docs/user-guide.md)。程式顯示資料來源、交易日及可用狀態；無法取得的市場數據不會以零代替。

已安裝早期版本的使用者，升級前請先關閉程式並備份應用資料。早期從其他更新來源取得版本者，需要先手動安裝一次本 repository 的版本。完整升級與資料確認步驟見 [使用說明](docs/user-guide.md)。

## 開發

需要 Node.js、Go 與桌面打包所需工具；完整環境和命令見 [開發文件](docs/development.md)。常用驗證入口：

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
```

```powershell
npm.cmd test
npm.cmd --workspace desktop test
npm.cmd run build:frontend
```

修改台股資料、來源或 PIT 行為前，請先讀 [台股開發權責](docs/TAIWAN_DEVELOPMENT_OWNERSHIP.md)；變更 API 時可參照 [後端路由](backend/docs/api-routes.md)。貢獻規範見 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 專案與授權狀態

本 repository 正進行 OSS 來源獨立化與既有功能維護。`TWstockfor_tick-stock-panel` 是新的台股核心功能主產品；mystocktracer 保留並維護自身的研究工作流與桌面使用者資料。每個來源候選與技術 gate 的最新判定見 [OSS 來源稽核](docs/oss-provenance-audit.md)。

mystocktracer 由 [jundizhou/easy-stock](https://github.com/jundizhou/easy-stock) 衍生。原作者署名、Git 歷史與相關 attribution 均保留；目前 [LICENSE](LICENSE) 是 PolyForm Noncommercial License 1.0.0。本 repository 現階段是 source-available，並非 OSI 定義的開源授權。任何未來授權變更都需要維護者明確決定。

如需回報問題，請使用 [Issue 模板](https://github.com/nanachi1212/mystocktracer/issues/new/choose)。安全問題請走 [私密回報管道](SECURITY.md)，勿公開憑證、本機資料或持倉內容。
