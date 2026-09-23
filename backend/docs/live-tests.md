# 可選的真實來源測試

一般 `go test ./...` 使用合成或本機 fixture，不應依賴外部台股服務。需要檢查當前官方端點時，在 PowerShell 明確啟用對應套件：

```powershell
cd backend
$env:EASY_STOCK_TW_LIVE_TEST = '1'
go test ./internal/providers/taiwan -run Live -v
Remove-Item Env:EASY_STOCK_TW_LIVE_TEST
```

ToAlpha 的既有 opt-in 變數仍保留舊名稱，這是測試入口相容識別，不是 canonical 產品名稱：

```powershell
cd backend
$env:A_STOCK_LIVE_TEST = '1'
go test ./internal/providers/toalpha -run Live -v
Remove-Item Env:A_STOCK_LIVE_TEST
```

台股套件覆蓋官方名錄、行情、K 線單位與新鮮度、基本面、廣度、情緒、產業雷達及個股證據。ToAlpha 套件檢查 MOPS 重大訊息的可選補充。外部端點失敗可能是流量限制、網路阻斷或來源 schema 變動；請區分來源健康與本機 deterministic regression，並記錄實際來源、時間與錯誤類別。
