# go-slog-datadog

[zozo advent calendar 2023](https://qiita.com/advent-calendar/2023/zozo) #2 執筆用レポジトリ

## 準備
```bash
# datadog agentを起動
docker compose build --no-cache
docker compose up -d
```

## ログ出力
```yaml
curl -X GET http://localhost:8080/
```
