package database

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5/pgtype"

	"waste-routing/internal/models"
)

const upsertWasteSourceSQL = `
INSERT INTO waste_routing.waste_sources (
    id,
    service_zone,
    address,
    district,
    waste_type,
    container_type,
    container_volume_m3,
    container_count,
    waste_amount,
    pickups_warm_period,
    pickups_cold_period,
    geom
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
    ST_SetSRID(ST_MakePoint($13, $12), 4326)
)
ON CONFLICT (id) DO UPDATE SET
    service_zone = EXCLUDED.service_zone,
    address = EXCLUDED.address,
    district = EXCLUDED.district,
    waste_type = EXCLUDED.waste_type,
    container_type = EXCLUDED.container_type,
    container_volume_m3 = EXCLUDED.container_volume_m3,
    container_count = EXCLUDED.container_count,
    waste_amount = EXCLUDED.waste_amount,
    pickups_warm_period = EXCLUDED.pickups_warm_period,
    pickups_cold_period = EXCLUDED.pickups_cold_period,
    geom = EXCLUDED.geom`

const selectWasteSourceSQL = `
SELECT
    id,
    service_zone,
    address,
    district,
    waste_type,
    container_type,
    container_volume_m3,
    container_count,
    waste_amount,
    pickups_warm_period,
    pickups_cold_period,
    ST_Y(geom),
    ST_X(geom)
FROM waste_routing.waste_sources`

const upsertTransferStationSQL = `
INSERT INTO waste_routing.transfer_stations (
    id,
    name,
    capacity,
    geom
) VALUES (
    $1, $2, $3,
    ST_SetSRID(ST_MakePoint($5, $4), 4326)
)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    capacity = EXCLUDED.capacity,
    geom = EXCLUDED.geom`

const selectTransferStationSQL = `
SELECT
    id,
    name,
    capacity,
    ST_Y(geom),
    ST_X(geom)
FROM waste_routing.transfer_stations`

const upsertPlantSQL = `
INSERT INTO waste_routing.plants (
    id,
    name,
    capacity,
    geom
) VALUES (
    $1, $2, $3,
    ST_SetSRID(ST_MakePoint($5, $4), 4326)
)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    capacity = EXCLUDED.capacity,
    geom = EXCLUDED.geom`

const selectPlantSQL = `
SELECT
    id,
    name,
    capacity,
    ST_Y(geom),
    ST_X(geom)
FROM waste_routing.plants`

type rowScanner interface {
	Scan(destinations ...any) error
}

// UpsertWasteSource inserts a waste source or updates the row with the same ID.
func (repository *Repository) UpsertWasteSource(ctx context.Context, source models.WasteSource) error {
	if _, err := repository.pool.Exec(ctx, upsertWasteSourceSQL, wasteSourceArguments(source)...); err != nil {
		return fmt.Errorf("upsert waste source %q: %w", source.ID, err)
	}
	return nil
}

// UpsertWasteSources writes all waste sources atomically in bounded batches.
func (repository *Repository) UpsertWasteSources(ctx context.Context, sources []models.WasteSource) error {
	return repository.executeBatchTransaction(ctx, "upsert waste sources", len(sources), func(index int) (batchStatement, error) {
		source := sources[index]
		return batchStatement{
			query: upsertWasteSourceSQL,
			args:  wasteSourceArguments(source),
		}, nil
	})
}

// GetWasteSource returns a waste source by its subject-domain ID.
func (repository *Repository) GetWasteSource(ctx context.Context, id string) (models.WasteSource, error) {
	row := repository.pool.QueryRow(ctx, selectWasteSourceSQL+` WHERE id = $1`, id)
	source, err := scanWasteSource(row)
	if err != nil {
		return models.WasteSource{}, mapRowError(fmt.Sprintf("get waste source %q", id), err)
	}
	return source, nil
}

// ListWasteSources returns all waste sources in stable ID order.
func (repository *Repository) ListWasteSources(ctx context.Context) ([]models.WasteSource, error) {
	rows, err := repository.pool.Query(ctx, selectWasteSourceSQL+` ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list waste sources: query: %w", err)
	}
	defer rows.Close()

	sources := make([]models.WasteSource, 0)
	for rows.Next() {
		source, err := scanWasteSource(rows)
		if err != nil {
			return nil, fmt.Errorf("list waste sources: scan row: %w", err)
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list waste sources: iterate rows: %w", err)
	}
	return sources, nil
}

// UpsertTransferStation inserts a transfer station or updates the row with the same ID.
func (repository *Repository) UpsertTransferStation(ctx context.Context, station models.TransferStation) error {
	if _, err := repository.pool.Exec(ctx, upsertTransferStationSQL, transferStationArguments(station)...); err != nil {
		return fmt.Errorf("upsert transfer station %q: %w", station.ID, err)
	}
	return nil
}

// UpsertTransferStations writes all transfer stations atomically in bounded batches.
func (repository *Repository) UpsertTransferStations(ctx context.Context, stations []models.TransferStation) error {
	return repository.executeBatchTransaction(ctx, "upsert transfer stations", len(stations), func(index int) (batchStatement, error) {
		station := stations[index]
		return batchStatement{
			query: upsertTransferStationSQL,
			args:  transferStationArguments(station),
		}, nil
	})
}

// GetTransferStation returns a transfer station by ID.
func (repository *Repository) GetTransferStation(ctx context.Context, id string) (models.TransferStation, error) {
	row := repository.pool.QueryRow(ctx, selectTransferStationSQL+` WHERE id = $1`, id)
	station, err := scanTransferStation(row)
	if err != nil {
		return models.TransferStation{}, mapRowError(fmt.Sprintf("get transfer station %q", id), err)
	}
	return station, nil
}

// ListTransferStations returns all transfer stations in stable ID order.
func (repository *Repository) ListTransferStations(ctx context.Context) ([]models.TransferStation, error) {
	rows, err := repository.pool.Query(ctx, selectTransferStationSQL+` ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list transfer stations: query: %w", err)
	}
	defer rows.Close()

	stations := make([]models.TransferStation, 0)
	for rows.Next() {
		station, err := scanTransferStation(rows)
		if err != nil {
			return nil, fmt.Errorf("list transfer stations: scan row: %w", err)
		}
		stations = append(stations, station)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list transfer stations: iterate rows: %w", err)
	}
	return stations, nil
}

// UpsertPlant inserts a plant or updates the row with the same ID.
func (repository *Repository) UpsertPlant(ctx context.Context, plant models.Plant) error {
	if _, err := repository.pool.Exec(ctx, upsertPlantSQL, plantArguments(plant)...); err != nil {
		return fmt.Errorf("upsert plant %q: %w", plant.ID, err)
	}
	return nil
}

// UpsertPlants writes all plants atomically in bounded batches.
func (repository *Repository) UpsertPlants(ctx context.Context, plants []models.Plant) error {
	return repository.executeBatchTransaction(ctx, "upsert plants", len(plants), func(index int) (batchStatement, error) {
		plant := plants[index]
		return batchStatement{
			query: upsertPlantSQL,
			args:  plantArguments(plant),
		}, nil
	})
}

// GetPlant returns a plant by ID.
func (repository *Repository) GetPlant(ctx context.Context, id string) (models.Plant, error) {
	row := repository.pool.QueryRow(ctx, selectPlantSQL+` WHERE id = $1`, id)
	plant, err := scanPlant(row)
	if err != nil {
		return models.Plant{}, mapRowError(fmt.Sprintf("get plant %q", id), err)
	}
	return plant, nil
}

// ListPlants returns all plants in stable ID order.
func (repository *Repository) ListPlants(ctx context.Context) ([]models.Plant, error) {
	rows, err := repository.pool.Query(ctx, selectPlantSQL+` ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list plants: query: %w", err)
	}
	defer rows.Close()

	plants := make([]models.Plant, 0)
	for rows.Next() {
		plant, err := scanPlant(rows)
		if err != nil {
			return nil, fmt.Errorf("list plants: scan row: %w", err)
		}
		plants = append(plants, plant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list plants: iterate rows: %w", err)
	}
	return plants, nil
}

func wasteSourceArguments(source models.WasteSource) []any {
	return []any{
		source.ID,
		source.ServiceZone,
		source.Address,
		source.District,
		source.WasteType,
		source.ContainerType,
		source.ContainerVolumeM3,
		source.ContainerCount,
		source.WasteAmount,
		nullablePickup(source.PickupsWarmPeriod),
		nullablePickup(source.PickupsColdPeriod),
		source.Latitude,
		source.Longitude,
	}
}

func transferStationArguments(station models.TransferStation) []any {
	return []any{
		station.ID,
		station.Name,
		station.Capacity,
		station.Latitude,
		station.Longitude,
	}
}

func plantArguments(plant models.Plant) []any {
	return []any{
		plant.ID,
		plant.Name,
		plant.Capacity,
		plant.Latitude,
		plant.Longitude,
	}
}

func nullablePickup(value float64) any {
	if math.IsNaN(value) {
		return nil
	}
	return value
}

func scanWasteSource(scanner rowScanner) (models.WasteSource, error) {
	var source models.WasteSource
	var warmPickups pgtype.Float8
	var coldPickups pgtype.Float8
	if err := scanner.Scan(
		&source.ID,
		&source.ServiceZone,
		&source.Address,
		&source.District,
		&source.WasteType,
		&source.ContainerType,
		&source.ContainerVolumeM3,
		&source.ContainerCount,
		&source.WasteAmount,
		&warmPickups,
		&coldPickups,
		&source.Latitude,
		&source.Longitude,
	); err != nil {
		return models.WasteSource{}, err
	}

	source.PickupsWarmPeriod = nullableFloatValue(warmPickups)
	source.PickupsColdPeriod = nullableFloatValue(coldPickups)
	return source, nil
}

func scanTransferStation(scanner rowScanner) (models.TransferStation, error) {
	var station models.TransferStation
	if err := scanner.Scan(
		&station.ID,
		&station.Name,
		&station.Capacity,
		&station.Latitude,
		&station.Longitude,
	); err != nil {
		return models.TransferStation{}, err
	}
	return station, nil
}

func scanPlant(scanner rowScanner) (models.Plant, error) {
	var plant models.Plant
	if err := scanner.Scan(
		&plant.ID,
		&plant.Name,
		&plant.Capacity,
		&plant.Latitude,
		&plant.Longitude,
	); err != nil {
		return models.Plant{}, err
	}
	return plant, nil
}

func nullableFloatValue(value pgtype.Float8) float64 {
	if !value.Valid {
		return math.NaN()
	}
	return value.Float64
}
