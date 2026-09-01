# Развертывание PostgreSQL/PostGIS

## Границы этапа

Compose-конфигурация запускает только PostgreSQL/PostGIS. Valhalla остается
внешним сервисом на `http://localhost:8002`, поэтому ее образ, тайлы, порт,
контейнер и способ запуска не меняются. Go- и MATLAB-код к БД пока не
подключаются, существующий файловый сценарий продолжает работать.

## Конфигурация

Параметры подключения задаются в локальном файле `code/.env`:

| Переменная | Назначение | Значение в примере |
|---|---|---|
| `POSTGRES_HOST` | Адрес БД для программ на хосте | `127.0.0.1` |
| `POSTGRES_PORT` | Опубликованный порт на хосте | `55432` |
| `POSTGRES_DB` | Имя БД | `waste_routing` |
| `POSTGRES_USER` | Владелец БД | `waste_routing` |
| `POSTGRES_PASSWORD` | Пароль владельца | в шаблоне не задан |
| `POSTGRES_SSLMODE` | Режим SSL для будущего Go-клиента | `disable` |

Порт `55432` выбран по умолчанию, чтобы не конфликтовать с локальной
установкой PostgreSQL на стандартном порту `5432`. Публикация привязана к
`127.0.0.1`, то есть БД не открыта в локальную сеть. Для контейнеров внутри
Compose адресом БД был бы `postgres:5432`; на этом этапе других контейнеров-
клиентов нет.

Файл `.env` игнорируется Git. В репозитории хранится только `.env.example`
с пустым паролем. Compose завершится с понятной ошибкой, если пароль не
задан.

## Первоначальный запуск

Нужны Docker Engine и Docker Compose. Из папки `code` выполните:

```powershell
Copy-Item .env.example .env
```

Затем задайте в `.env` уникальный `POSTGRES_PASSWORD` и запустите БД:

```powershell
docker compose config --quiet
docker compose up -d postgres
docker compose ps
```

Именованный volume `waste-routing_postgres_data` сохраняет данные между
перезапусками контейнера.

## Применение миграции

Миграция `000001_initial_schema.sql` смонтирована read-only как отдельный
файл `/docker-entrypoint-initdb.d/20_000001_initial_schema.sql`. Это сохраняет
встроенные init-скрипты PostGIS-образа. При первой инициализации пустого
volume PostgreSQL выполняет init-файлы в лексикографическом порядке. Миграция:

- подключает extension `postgis`;
- создает схему `waste_routing` и семь предметных таблиц;
- создает PK, FK, `CHECK`, `UNIQUE`, B-tree и GiST-индексы.

Init-скрипты выполняются только для нового пустого volume. Уже примененную
версионную миграцию не следует запускать повторно. Механизм последовательного
применения будущих миграций потребуется добавить вместе со следующими
версиями схемы; для текущей единственной начальной миграции используется
штатный init-механизм образа PostgreSQL.

## Проверки

Состояние healthcheck:

```powershell
$dbUser = (docker compose exec -T postgres printenv POSTGRES_USER).Trim()
$dbName = (docker compose exec -T postgres printenv POSTGRES_DB).Trim()

docker compose ps
docker compose exec -T postgres pg_isready `
  --host 127.0.0.1 `
  --username $dbUser `
  --dbname $dbName
Test-NetConnection -ComputerName 127.0.0.1 -Port 55432
```

Версия PostGIS:

```powershell
docker compose exec -T postgres psql `
  --set=ON_ERROR_STOP=1 `
  --username $dbUser `
  --dbname $dbName `
  --command "SELECT PostGIS_Full_Version();"
```

Созданные таблицы:

```powershell
docker compose exec -T postgres psql `
  --set=ON_ERROR_STOP=1 `
  --username $dbUser `
  --dbname $dbName `
  --command "SELECT table_name FROM information_schema.tables WHERE table_schema = 'waste_routing' ORDER BY table_name;"
```

Созданные индексы, включая пространственные GiST-индексы:

```powershell
docker compose exec -T postgres psql `
  --set=ON_ERROR_STOP=1 `
  --username $dbUser `
  --dbname $dbName `
  --command "SELECT indexname, indexdef FROM pg_indexes WHERE schemaname = 'waste_routing' ORDER BY indexname;"
```

## Остановка

Остановить БД без удаления данных:

```powershell
docker compose stop postgres
```

Удалить контейнер и сеть, сохранив volume:

```powershell
docker compose down
```

Команда `docker compose down --volumes` также необратимо удаляет локальную
БД и нужна только для осознанного создания чистого тестового окружения.
