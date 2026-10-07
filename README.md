# Fault-Tolerant Payment Processing Engine (Go Monorepo)

Высоконадежный отказоустойчивый платежный процессинг на Go.

## 🚀 Быстрый старт

### Вариант А: Всё в Docker (одной командой)

Запуск всей платформы (Postgres, Redis, API Gateway с UI, Outbox Worker) в изолированных контейнерах:

```bash
make docker-up
```
*(После запуска веб-панель сразу доступна на [http://localhost:8080](http://localhost:8080))*

Для просмотра логов:
```bash
make docker-logs
```

---

### Вариант Б: Локальная разработка на хосте (Go)

1. Запуск только баз данных (PostgreSQL + Redis):
```bash
make infra-up
```

2. Запуск сервисов:
```bash
# В терминале 1: API Gateway (раздает API и UI)
make run-gateway

# В терминале 2: Transactional Outbox воркер
make run-outbox
```

---

### Опционально: Полная телеметрия и Kafka UI (Redpanda, Jaeger, Prometheus)
```bash
make docker-full
```
- **Redpanda Console (Kafka UI)**: [http://localhost:8081](http://localhost:8081)
- **Jaeger UI (Tracing)**: [http://localhost:16686](http://localhost:16686)
- **Prometheus**: [http://localhost:9090](http://localhost:9090)

Проверка здоровья:
```bash
curl http://localhost:8080/healthz
```

Тестовый платеж с идемпотентным ключом:
```bash
curl -X POST http://localhost:8080/v1/payments \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: a1b2c3d4-e5f6-7890-abcd-ef1234567890" \
  -d '{
    "merchant_id": "00000000-0000-0000-0000-000000000001",
    "customer_id": "cust_123",
    "amount": 4999,
    "currency": "USD"
  }'
```

Повторная отправка того же запроса с тем же `Idempotency-Key` вернет кэшированный ответ с заголовком:
`Idempotent-Replayed: true` без повторного списания средств.

---

## 🛠 Реализованная оснастка (`pkg/`)

- [x] **`pkg/idempotency`**: Механизм предотвращения повторных списаний (Redis атомарные блокировки + in-progress лок + сохраненный ответ + HTTP Middleware).
- [x] **`pkg/outbox`**: Transactional Outbox pattern с безопасным параллельным поллингом через `SELECT ... FOR UPDATE SKIP LOCKED` и экспоненциальным backoff.
- [x] **`pkg/resilience`**: Circuit Breaker и Retry с Full Jitter для защиты от сбоев эквайеров и банков.
- [x] **`pkg/money`**: Безопасный денежный тип в минимальных единицах (cents/kopecks) — исключает ошибки округления `float`.
- [x] **`pkg/logger`**: PCI-DSS совместимый структурированный логгер (`slog`) с контекстной трассировкой (`trace_id`, `span_id`, `idempotency_key`) и автоматическим маскированием номеров карт (PAN) и CVV.
- [x] **`pkg/postgres`**: Пул соединений `pgxpool`, безопасный менеджер транзакций `WithTx` и healthcheck.
- [x] **`pkg/shutdown`**: Graceful shutdown координатор с таймаутами для чистой остановки сервисов и завершения активных транзакций.

Подробное описание архитектуры доступно в [docs/architecture.md](docs/architecture.md).
