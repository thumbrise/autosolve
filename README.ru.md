# autosolve

[![CI](https://github.com/thumbrise/autosolve/actions/workflows/ci.yml/badge.svg)](https://github.com/thumbrise/autosolve/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/thumbrise/autosolve.svg)](https://pkg.go.dev/github.com/thumbrise/autosolve)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](/LICENSE)

Доступные языки: [English](README.md), [Русский](README.ru.md)

Легковесный демон, который опрашивает GitHub-репозитории и запускает AI-агентов для автоматического решения задач.

> Исходная идея проекта: [docs/IDEA.md](docs/IDEA.md)

## Навигация

- [Review Guidelines](REVIEW.md) — стиль кода, архитектура, git-конвенции
- [Документация](docs) — дополнительные документы и переводы

## Стек технологий

| Компонент       | Технология                                                                        |
|-----------------|-----------------------------------------------------------------------------------|
| Язык            | Go                                                                                |
| База данных     | SQLite (pure Go, WAL mode)                                                        |
| Миграции        | [goose](https://github.com/pressly/goose)                                         |
| SQL-кодогенерация | [sqlc](https://sqlc.dev)                                                        |
| DI              | [Wire](https://github.com/google/wire)                                            |
| Observability   | [OpenTelemetry](https://opentelemetry.io) (traces, metrics, logs → OTLP/gRPC)     |
| CLI             | [cobra](https://github.com/spf13/cobra) + [viper](https://github.com/spf13/viper) |

## Быстрый старт

### Требования

- Go 1.26+
- [Task](https://taskfile.dev) (опционально, но рекомендуется)

### Установка

```bash
git clone https://github.com/thumbrise/autosolve.git
cd autosolve
go mod download
cp config.yml.example config.yml
```

Отредактируйте `config.yml` — как минимум укажите GitHub-токен и целевые репозитории:

```yaml
github:
  token: ghp_your_token_here
  repositories:
    - owner: your-org
      name: your-repo
```
<details>
  <summary>OpenTelemetry</summary>

**Отключён по умолчанию.**

Сбор данных [OpenTelemetry](https://opentelemetry.io) настраивается через конфигурационные переменные со стандартной OTEL-семантикой:

```yaml
otel:
  sdkDisabled: true
  serviceName: autosolve
  resourceAttributes: "service.version=1.0.0,deployment.environment=production"
  propagators: "tracecontext,baggage"
  traces:
    exporter: otlp
    sampler: parentbased_always_on
    samplerArg: "1.0"
  metrics:
    exporter: otlp
  logs:
    exporter: otlp
  exporter:
    endpoint: "localhost:4317"
    protocol: grpc
    headers: "uptrace-dsn=http://aiji-qvjRjFBnObLuzAkpA@localhost:14318?grpc=14317"
    timeout: 10s
```

Все поля конфигурации можно переопределить через переменные окружения с префиксом `AUTOSOLVE_`.
Пример: `otel.serviceName` → `AUTOSOLVE_OTEL_SERVICENAME`.

Подробнее: https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/

</details>

### Миграция базы данных

```bash
go run . migrate up -y
```

### Запуск планировщика

```bash
go run . schedule
```

Или через Task:

```bash
task up
```

## Архитектура

```
cmd/                    Точки входа CLI (cobra)
internal/
├── bootstrap/          Инициализация приложения (Bootstrap → Wire → Kernel)
├── config/             Типизированные конфиг-структуры (на базе viper)
├── domain/             Бизнес-логика
│   ├── issue/          Парсер задач (Worker)
│   ├── repository/     Валидатор репозиториев (Preflight)
│   └── spec/           Спецификации задач
│       └── tenants/    Определения тенантов (например RepoTenant)
├── application/        Слой оркестрации
│   ├── schedule.go     Двухфазный Scheduler
│   ├── planner.go      Планирование задач по репозиториям
│   ├── contracts.go    Интерфейсы Preflight / Worker
│   └── registry.go     Регистрация задач
└── infrastructure/     Внешние зависимости
    ├── config/         Загрузка конфигурации (viper reader, validator)
    ├── github/         GitHub API клиент + rate limiter
    ├── dal/            Слой доступа к данным
    │   ├── model/      Доменные модели
    │   ├── queries/    SQL-файлы (исходники для sqlc)
    │   ├── repositories/ Реализации репозиториев
    │   └── sqlcgen/    Сгенерированный sqlc-код
    ├── database/       SQLite-подключение + goose-мигратор
    ├── logger/         Настройка slog
    └── telemetry/      Инициализация OTEL SDK
pkg/
└── longrun/            Оркестрация задач с retry и exponential backoff
```

Планировщик работает в две фазы:

1. **Preflights** — одноразовые задачи (например, валидация доступа к репозиторию через GitHub API). Все должны пройти до запуска воркеров.
2. **Workers** — долгоживущие интервальные задачи (например, опрос и парсинг задач). Если любой воркер завершается с ошибкой окончательно, все остальные отменяются.

## Команды

| Команда                 | Описание                                            |
|-------------------------|-----------------------------------------------------|
| `schedule`              | Запустить демон опроса                              |
| `migrate up [N]`        | Применить ожидающие миграции (все по умолчанию)     |
| `migrate up:fresh`      | Удалить все таблицы и выполнить миграции заново     |
| `migrate down <N\|*>`   | Откатить N миграций (или все через `*`)             |
| `migrate status`        | Показать статус миграций                            |
| `migrate create <name>` | Создать новый SQL-файл миграции                     |
| `migrate redo`          | Откатить и заново применить последнюю миграцию      |

## Разработка

```bash
task generate   # sqlc + wire + лицензионные заголовки
task lint        # golangci-lint + license-eye + govulncheck + sqlfluff + проверка типов sqlcgen
task test        # юнит-тесты + бенчмарки
```

## Текущий статус

Epic v1 в процессе — см. [Epic: v1 architecture redesign](https://github.com/thumbrise/autosolve/issues/59).

Что работает сейчас:
- Опрос задач из нескольких GitHub-репозиториев с сохранением состояния в SQLite
- Preflight-валидация репозиториев
- Rate limiting через HTTP-транспорт
- goose-миграции + sqlc-генерированный DAL
- Полная OTEL-наблюдаемость (traces, metrics, logs)
- Двухфазный планировщик с retry и exponential backoff

## Лицензия

[Apache License 2.0](LICENSE)