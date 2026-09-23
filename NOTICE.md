# mystocktracer 授權與歷史來源

mystocktracer current codebase is MIT-licensed. Historical upstream easy-stock material and third-party components retain their applicable original licenses and attribution.

## 現行程式碼

維護者於 2026-09-23 明確決定，將 B6 完成獨立實作後的現行 mystocktracer 原創／重寫程式碼、測試、品牌素材與一般文件採用根目錄的 [MIT License](LICENSE)。著作權屬 nanachi1212 與相應的 mystocktracer contributors。本次授權變更從引入 MIT 的提交起適用；工程證據與決策範圍見 [OSS 來源稽核](docs/oss-provenance-audit.md)。

## easy-stock 歷史內容

本 repository 由 [jundizhou/easy-stock](https://github.com/jundizhou/easy-stock) 衍生，比對點為 `b969d05984dda736de9ee3dc2a881e8c65a373c5`。Git history、原作者著作權與 attribution 持續保留。

Copyright (c) 2026 jundizhou. All rights reserved.

下列為 upstream 原始 Required Notice，適用於其歷史 easy-stock 內容：

Required Notice: Copyright (c) 2026 jundizhou. Commercial use of easy-stock requires prior written permission from the copyright holder.

原 LICENSE（含完整 Required Notice 與 PolyForm Noncommercial License 1.0.0）原文保存在 [LICENSES/easy-stock-PolyForm-Noncommercial-1.0.0.txt](LICENSES/easy-stock-PolyForm-Noncommercial-1.0.0.txt)。[歷史 release notes](.github/release-notes) 中 v0.1.0、v0.2.0、v0.3.0、v0.4.0、v0.5.0、v0.5.1、v0.6.0、v0.6.1、v0.7.0、v0.8.0、v0.9.0、v0.9.1、v0.9.2 共 13 份 upstream 文件保持原文；它們是歷史紀錄，不是現行產品的授權宣告。

本次 MIT 決策不追溯重新授權 upstream 歷史 commits、上述 release notes 或其他第三方內容。歷史版本依各版本隨附的授權及原權利人適用的條款處理；曾適用 PolyForm Noncommercial 的內容仍受該授權約束，不因根目錄 LICENSE 改為 MIT 而取得 MIT 權利。更早版本也不因本次變更而被改套 PolyForm。

## 第三方元件與資料

依賴、bundled runtimes、第三方素材及外部市場資料各自的著作權、授權與使用條款持續適用。MIT 僅描述現行 mystocktracer 自有內容，不替代這些條款。已整理的通知見 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)，其餘依各元件附帶的 LICENSE／NOTICE；該通知文件不是完整依賴授權清單。依賴版本以 `package-lock.json`、`backend/go.mod`／`go.sum` 及封裝的 `runtime-manifest.json` 為準。
