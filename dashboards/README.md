# dashboards

Per-module HTML dashboards для pipeline e_zoo. FastAPI + Jinja2, port `8091`.

## Что показывает

Index (`/`) — 7 карточек модулей с headline-метрикой.

Per-module страницы (`/m1` ... `/m7`):
- **Input** — что и откуда читает модуль (HTTP-источник или таблицы БД)
- **Output** — что и куда пишет (таблицы БД или HTTP-эндпоинты)
- **Sample rows** — 10 свежих строк из ключевых таблиц
- **Extras** — последний run, snapshot pointer, распределения KPI

Авто-обновление каждые 30 секунд через `<meta http-equiv="refresh">`.

## Запуск

После полного e2e:
```
make e2e-up
open http://localhost:8091
```

Минимальный запуск (только postgres + mock-erp + dashboards):
```
docker compose up -d --build postgres migrate mock-erp dashboards
curl -fsS http://localhost:8091/healthz
open http://localhost:8091
```

## ENV vars

| Var | Default | Description |
|-----|---------|-------------|
| `DASHBOARDS_DSN` | `postgres://e_zoo_app:ezoo_app_dev@postgres:5432/source_adapter` | DSN до общей БД |
| `MOCK_ERP_URL` | `http://mock-erp:8090` | URL mock-erp для M1/M7 |
| `MOCK_ERP_API_KEY` | `test-api-key` | X-API-Key для mock-erp |
| `DATA_MARTS_URL` | `http://data-marts:8082` | URL data-marts API для M3 |
| `JWT_SECRET` | `dev-secret-change-in-prod` | HS256 секрет для JWT |
| `JWT_ROLE` | `it-read` | Роль JWT (issuer claim) |
| `HTTP_TIMEOUT_SEC` | `5.0` | Таймаут HTTP-запросов |

## Поведение на пустой БД

Все queries обёрнуты в try/except — если таблицы ещё не созданы или БД пуста,
dashboards показывает "0 rows" / "no data" / "n/a", не падая 500.

## Добавление нового модуля

1. Добавить queries в `app/queries.py`
2. Добавить запись в `MODULES` в `app/main.py`
3. Создать handler `@app.get("/m8")` следуя шаблону существующих модулей
4. Добавить ссылку в `header nav` в `app/templates/base.html`

## Файловая структура

```
dashboards/
├── Dockerfile
├── requirements.txt
├── README.md
└── app/
    ├── __init__.py
    ├── main.py          # FastAPI + 8 routes + Jinja2
    ├── db.py            # psycopg pool, graceful empty-result on errors
    ├── queries.py       # все SQL queries в одном месте
    └── templates/
        ├── base.html
        ├── index.html
        ├── module.html
        └── partials/
            └── kv_table.html
```
