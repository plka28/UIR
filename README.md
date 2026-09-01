# Waste Routing Optimization

## Назначение Проекта

Проект строит полный расчетный конвейер для распределения мест накопления отходов между мусороперегрузочными станциями и заводами переработки:

1. подготовка исходных таблиц;
2. расчет дорожных расстояний через локальную Valhalla;
3. решение задачи оптимизации в MATLAB через `intlinprog`;
4. экспорт точечных и линейных GeoJSON-слоев для QGIS.

Рабочая папка проекта - `code`. Все команды ниже выполняются из нее, если не указано иначе.

## Структура Проекта

```text
README.md
code/
  go.mod
  go.sum

  cmd/
    prepare-input/      # подготовка исходных XLSX в CSV/JSON
    valhalla-matrix/    # расчет матриц расстояний через Valhalla
    qgis-export/        # экспорт GeoJSON-слоев для QGIS

  internal/
    database/           # подключение PostgreSQL и repository для сущностей
    input/              # чтение, валидация и запись подготовленных данных
    models/             # общие Go-модели предметной области
    qgis/               # сборка GeoJSON FeatureCollection
    utils/              # общие CSV/JSON helpers
    valhalla/           # HTTP-клиент Valhalla и matrix API

  matlab/
    run_optimization.m
    load_input_data.m
    validate_input_data.m
    create_variable_maps.m
    build_problem.m
    solve_problem.m
    decode_solution.m
    save_results.m

  data/
    prepared/           # подготовленные входы для Valhalla и MATLAB
    raw/                # место для исходных данных, если нужно хранить их внутри code

  output/
    matrices/           # матрицы расстояний Valhalla
    optimization/       # результаты MATLAB-оптимизации
    qgis/               # GeoJSON-слои для QGIS
```

## Требования

- Go: версия из `go.mod` (`go 1.24.0`) или совместимая.
- MATLAB с Optimization Toolbox: нужен `intlinprog`.
- Docker: для запуска Valhalla-контейнера и PostgreSQL/PostGIS.
- Valhalla: локальный HTTP API, по умолчанию `http://localhost:8002`.
- QGIS: для открытия итоговых GeoJSON-слоев.

Изолированное развертывание PostgreSQL/PostGIS описано в
[`docs/database_deployment.md`](code/docs/database_deployment.md). Оно не меняет
существующий файловый сценарий и внешний контейнер Valhalla.

Проверка Go-части:

```powershell
go test ./...
go build ./...
```

## Формат Входных Данных

`prepare-input` читает исходные XLSX-таблицы:

- таблица мусорок: id площадки, адрес, долгота, широта, объем контейнера, количество контейнеров, число вывозов в месяц для теплого и холодного периода;
- таблица МПС: id, наименование, мощность в тыс. тонн/год, широта, долгота;
- таблица заводов/КПО: id, наименование, мощность в тыс. тонн/год, широта, долгота.

По умолчанию команда ожидает найденный рядом с проектом набор:

```text
../Данные_Сергиево-Посадская/Сергиево-Посадская_площадки.xlsx
../Данные_Сергиево-Посадская/МПС.xlsx
```

После подготовки MATLAB и последующие Go-команды работают с CSV:

`data/prepared/waste_sources.csv`:

```text
id,service_zone,address,district,waste_type,container_type,container_volume_m3,container_count,waste_amount,pickups_warm_period,pickups_cold_period,latitude,longitude
```

`data/prepared/transfer_stations.csv` и `data/prepared/plants.csv`:

```text
id,name,capacity,latitude,longitude
```

Координаты должны быть в WGS84: `latitude` от -90 до 90, `longitude` от -180 до 180. Объемы отходов и мощности должны быть неотрицательными.

`waste_amount` сохраняется в тоннах в год. Расчет выполняется по формуле:

```text
waste_amount = container_volume_m3 * container_count *
               (pickups_warm_period * 6.258 + pickups_cold_period * 5.7) *
               0.2
```

где `6.258` - коэффициент теплого периода, `5.7` - коэффициент холодного периода, а `0.2` - средняя плотность отходов в тоннах на кубический метр. Мощности МПС и заводов из исходных таблиц переводятся из тыс. тонн/год в тонны/год умножением на `1000`.

## Порядок Запуска

### 1. Подготовка Исходных Данных

```powershell
go run ./cmd/prepare-input
```

С явными путями:

```powershell
go run ./cmd/prepare-input `
  -sources "../Данные_Сергиево-Посадская/Сергиево-Посадская_площадки.xlsx" `
  -sources-sheet "Sheet1" `
  -transfers "../Данные_Сергиево-Посадская/МПС.xlsx" `
  -transfers-sheet "МПС" `
  -plants "../Данные_Сергиево-Посадская/МПС.xlsx" `
  -plants-sheet "КПО" `
  -out "data/prepared"
```

### 2. Запуск Valhalla

Проект не создает дорожные тайлы и не содержит готовый Docker-образ Valhalla. Нужно запустить внешний контейнер или локальный сервис Valhalla так, чтобы API был доступен на `http://localhost:8002`.

Общая схема:

```powershell
docker run --rm -p 8002:8002 <valhalla-image-with-prepared-tiles>
```

Используемый endpoint Valhalla: `/sources_to_targets`.

### 3. Расчет Матриц

```powershell
go run ./cmd/valhalla-matrix -base-url "http://localhost:8002"
```

С явными путями:

```powershell
go run ./cmd/valhalla-matrix `
  -base-url "http://localhost:8002" `
  -sources "data/prepared/waste_sources.csv" `
  -transfers "data/prepared/transfer_stations.csv" `
  -plants "data/prepared/plants.csv" `
  -out "output/matrices" `
  -batch-size 100 `
  -timeout 120
```

### 4. Запуск MATLAB-Оптимизации

Из папки `code/matlab`:

```powershell
matlab -batch "result = run_optimization();"
```

Или в MATLAB:

```matlab
cd('path/to/code/matlab')
result = run_optimization();
```

Скрипт выполняет полный цикл: загрузка подготовленных CSV и матриц, фильтрация мусорок без доступного маршрута, проверка данных, построение MILP-задачи, запуск `intlinprog`, декодирование решения и сохранение CSV-результатов.

Перед построением задачи MATLAB исключает только мусорки, у которых нет ни одного доступного маршрута. Подмножество по мощности не формируется: проверка мощностей выполняется ограничениями оптимизационной модели. Недоступные направления из матриц Valhalla не используются в оптимизации, соответствующие переменные получают верхнюю границу `0`.

Для `intlinprog` используется алгоритм `legacy`. В MATLAB R2024a алгоритм `highs` может аварийно завершаться на большой бинарной модели во время presolve.

### 5. Экспорт GeoJSON

После MATLAB-расчета:

```powershell
go run ./cmd/qgis-export
```

С явными путями:

```powershell
go run ./cmd/qgis-export `
  -sources "data/prepared/waste_sources.csv" `
  -transfers "data/prepared/transfer_stations.csv" `
  -plants "data/prepared/plants.csv" `
  -source-assignments "output/optimization/source_assignments.csv" `
  -transfer-flows "output/optimization/transfer_flows.csv" `
  -out "output/qgis"
```

Команда сохраняет GeoJSON-слои и одноименные QGIS-стили `.qml`. Стили задают:

- заводы/КПО - самые крупные маркеры-звезды;
- МПС - треугольники меньшего размера;
- мусорки - маленькие точки;
- уникальный цвет для каждой МПС и каждого КПО;
- цвет мусорки по объекту, на который она назначена.

### 6. Открытие Результатов В QGIS

В QGIS выберите `Layer -> Add Layer -> Add Vector Layer`, затем добавьте файлы из `code/output/qgis`.

Рекомендуемый порядок слоев:

1. `waste_sources.geojson`;
2. `transfer_stations.geojson`;
3. `plants.geojson`;
4. `source_assignments.geojson`;
5. `transfer_to_plant_flows.geojson`.

Для оформления слоя откройте его свойства, перейдите в `Symbology`, выберите `Style -> Load Style`, затем загрузите одноименный файл `.qml` из `output/qgis`:

- `waste_sources.qml` для `waste_sources.geojson`;
- `transfer_stations.qml` для `transfer_stations.geojson`;
- `plants.qml` для `plants.geojson`;
- `source_assignments.qml` для `source_assignments.geojson`;
- `transfer_to_plant_flows.qml` для `transfer_to_plant_flows.geojson`.

Точечные слои показывают объекты инфраструктуры, линейные слои показывают назначения мусорок и потоки МПС -> завод.

## Выходные Файлы

### Prepared CSV/JSON

Папка: `data/prepared`.

- `waste_sources.csv` - нормализованные мусорки.
- `transfer_stations.csv` - нормализованные МПС.
- `plants.csv` - нормализованные заводы.
- `points.json` - общий список точек для контроля и интеграции.
- `validation_report.json` - отчет по входной валидации.

### Matrices CSV

Папка: `output/matrices`.

- `source_to_transfer.csv` - расстояния мусорка -> МПС.
- `source_to_plant.csv` - расстояния мусорка -> завод.
- `transfer_to_plant.csv` - расстояния МПС -> завод.

Формат матриц: первая колонка `id` содержит исходный объект, остальные колонки названы id целевых объектов, значения в ячейках - расстояния в километрах. Недоступные направления сохраняются как `NaN`.

### Optimization CSV

Папка: `output/optimization`.

- `source_assignments.csv` - назначение каждой мусорки.
  Колонки: `source_id`, `assignment_type`, `destination_id`, `destination_name`, `waste_amount`, `distance_km`, `transport_cost`.
- `transfer_flows.csv` - потоки от МПС к заводам.
  Колонки: `transfer_station_id`, `transfer_station_name`, `plant_id`, `plant_name`, `waste_amount`, `distance_km`, `transport_cost`.
- `summary.csv` - сводные показатели решения.

### QGIS GeoJSON

Папка: `output/qgis`.

- `waste_sources.geojson` - точки мусорок.
- `transfer_stations.geojson` - точки МПС.
- `plants.geojson` - точки заводов.
- `source_assignments.geojson` - линии от мусорок до назначенных МПС или заводов.
- `transfer_to_plant_flows.geojson` - линии от МПС до заводов.
- `waste_sources.qml`, `transfer_stations.qml`, `plants.qml`, `source_assignments.qml`, `transfer_to_plant_flows.qml` - готовые стили слоев для QGIS.

Все GeoJSON-файлы имеют формат `FeatureCollection`. Координаты записываются в порядке GeoJSON: `[longitude, latitude]`.

## Математическая Модель

Модель решается в MATLAB как MILP через `intlinprog`.

Переменные:

- `x_pt(p,t)` - бинарная переменная: мусорка `p` назначена на МПС `t`;
- `x_pu(p,u)` - бинарная переменная: мусорка `p` назначена напрямую на завод `u`;
- `y_tu(t,u)` - непрерывная неотрицательная переменная: объем отходов с МПС `t` на завод `u`.

Ограничения:

- каждая мусорка назначается ровно один раз;
- мощность каждой МПС не превышается;
- поток через каждую МПС сохраняется;
- мощность каждого завода не превышается;
- `x_pt` и `x_pu` бинарные;
- `y_tu` непрерывные и неотрицательные.

Целевая функция минимизирует суммарные транспортные затраты:

- мусорка -> МПС;
- мусорка -> завод;
- МПС -> завод.

В текущей реализации расстояния из матриц используются как транспортные стоимости, а участки от мусорок дополнительно взвешиваются объемом отходов.

## Возможные Ошибки

`missing required columns`
: Проверьте заголовки входных CSV/XLSX. Для подготовленных CSV обязательны `id`, `latitude`, `longitude`, а для МПС/заводов также `capacity`.

`open ... The system cannot find the file specified`
: Команда не нашла входной файл. Проверьте текущую папку запуска (`code`) и пути во флагах `-sources`, `-transfers`, `-plants`, `-source-assignments`, `-transfer-flows`.

`Valhalla request ... connectex ... refused` или таймаут
: Valhalla не запущена, запущена на другом порту или `-base-url` указан неверно. Проверьте Docker-контейнер и доступность `http://localhost:8002`.

`Valhalla response has unexpected matrix size`
: Ответ Valhalla не соответствует числу источников и целей. Проверьте версию сервиса, endpoint `/sources_to_targets` и JSON-ответ.

`NaN or infinite values ... objective`
: В старой версии расчета это означало, что в матрицах есть недоступные направления. Текущая MATLAB-часть запрещает такие направления автоматически, но если после фильтрации у мусорки не остается ни одного маршрута, она исключается из расчетного набора.

`Total plant capacity ... lower than total waste amount`
: Суммарная мощность заводов ниже годового объема отходов. Проверьте, что после `prepare-input` `waste_amount` выражен в тоннах/год, а мощности МПС и заводов переведены в тонны/год.

`output/optimization/source_assignments.csv` не найден при `qgis-export`
: Сначала запустите MATLAB-оптимизацию через `run_optimization.m`.

`intlinprog` не найден
: В MATLAB отсутствует Optimization Toolbox или он не активирован.

`Access violation` в `highs.dll`
: Это сбой решателя HiGHS в MATLAB R2024a на большой MILP-задаче. В проекте `solve_problem.m` переключает `intlinprog` на алгоритм `legacy`; если сообщение осталось, проверьте, что используется актуальная версия файлов из `code/matlab`.

## Полезные Команды

```powershell
go run ./cmd/prepare-input -h
go run ./cmd/valhalla-matrix -h
go run ./cmd/qgis-export -h
go test ./...
go build ./...
```
