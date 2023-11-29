# go-slog-datadog

## 準備
```bash
# datadog agentを起動
API_KEY=xxxx
docker run -d --cgroupns host \
              --pid host \
              -v /var/run/docker.sock:/var/run/docker.sock:ro \
              -v /proc/:/host/proc/:ro \
              -v /sys/fs/cgroup/:/host/sys/fs/cgroup:ro \
              -p 127.0.0.1:8126:8126/tcp \
              -e DD_API_KEY=$API_KEY \
              -e DD_APM_ENABLED=true \
              -e DD_SITE="ap1.datadoghq.com" \
              gcr.io/datadoghq/agent:latest

# アプリケーションを起動
go run main.go
```
