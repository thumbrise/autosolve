# autosolve

[![CI](https://github.com/thumbrise/autosolve/actions/workflows/ci.yml/badge.svg)](https://github.com/thumbrise/autosolve/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/thumbrise/autosolve.svg)](https://pkg.go.dev/github.com/thumbrise/autosolve)
[![GitHub stars](https://img.shields.io/github/stars/thumbrise/autosolve?style=social)](https://github.com/thumbrise/autosolve/stargazers)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](/LICENSE)

Доступные языки: [English](README.md), [Русский](README.ru.md)

Self-hosted Go-демон, который опрашивает GitHub-репозитории и запускает AI-агентов для автоматического решения задач. Без вебхуков, без CI-костылей — просто запусти и забудь.

> **🚧 Активная разработка.** Базовая инфраструктура работает. Слой запуска AI-агентов — следующий этап. [Контрибуция приветствуется.](https://thumbrise.github.io/autosolve/contributing/adding-worker)

## Быстрый старт

```bash
git clone https://github.com/thumbrise/autosolve.git && cd autosolve
go mod download
cp config.yml.example config.yml   # укажи токен + репозитории
go run . migrate up -y
go run . schedule
```

## Документация

📖 **[thumbrise.github.io/autosolve](https://thumbrise.github.io/autosolve)** — полная документация, гайды, архитектура, девлог.

| Раздел | Что внутри |
|--------|-----------|
| [Быстрый старт](https://thumbrise.github.io/autosolve/guide/getting-started) | Настройка за 5 минут |
| [Конфигурация](https://thumbrise.github.io/autosolve/guide/configuration) | Все параметры |
| [Архитектура](https://thumbrise.github.io/autosolve/internals/overview) | Как устроена система |
| [Идея проекта](https://thumbrise.github.io/autosolve/project/idea) | Зачем этот проект |
| [Контрибуция](https://thumbrise.github.io/autosolve/contributing/adding-worker) | Добавь воркер за 4 шага |
| [Девлог](https://thumbrise.github.io/autosolve/devlog/) | Как мы к этому пришли — дневник решений |

## Текущий статус

Epic v1 в процессе — см. [Epic: v1 architecture redesign](https://github.com/thumbrise/autosolve/issues/59).

Что работает: мультирепо-поллинг, двухфазный планировщик, per-error retry с degraded mode, rate limiting, полная OTEL-наблюдаемость, SQLite с goose + sqlc.

Что дальше: движок правил для запуска AI, публикация результатов обратно в GitHub, адаптивный поллинг.

## Лицензия

[Apache License 2.0](LICENSE)