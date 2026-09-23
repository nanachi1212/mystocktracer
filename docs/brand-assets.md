# mystocktracer 品牌素材

本輪在此 repository 建立全新幾何設計：深藍圓角底、研究追蹤圓環、薄荷綠折線與四個金色觀測點。未讀取、描摹或重用 easy-stock、OpenStock 或其它品牌圖片。

canonical deterministic source 為 `desktop/scripts/generate-brand.mjs`，對應向量來源 `desktop/assets/mystocktracer-mark.svg`。只依賴 Node 標準庫；執行 `node desktop/scripts/generate-brand.mjs` 可產生完全一致的 PNG、ICO、ICNS 與 frontend mark。ICO 內嵌 256px PNG；ICNS 包含 512px ic09。

素材檔與腳本均隨 repository 保存，build runner 不需外部轉檔 binary。B4 素材替換本身未修改 root LICENSE；後續維護者已將現行原創素材與程式碼一併採用 [MIT](../LICENSE)，範圍見 [NOTICE](../NOTICE.md)。
