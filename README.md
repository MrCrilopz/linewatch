# linewatch

API en Go para vigilar medidores y explicar anomalías. Esta fase solo responde el health check.

```bash
go test ./...
go run ./cmd/linewatch
```

`GET http://localhost:8080/health` → `{"status":"ok"}`.
