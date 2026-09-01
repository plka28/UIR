package database

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"waste-routing/internal/models"
)

const routeCostSelectColumns = `
SELECT
    route_cost_id,
    source_type,
    COALESCE(source_waste_source_id, source_transfer_station_id),
    target_type,
    COALESCE(target_transfer_station_id, target_plant_id),
    costing_model,
    distance_km,
    travel_time_seconds,
    reachable,
    status,
    error_message,
    calculated_at
FROM waste_routing.route_costs`

const upsertSourceToTransferRouteSQL = `
INSERT INTO waste_routing.route_costs (
    source_waste_source_id,
    target_transfer_station_id,
    costing_model,
    distance_km,
    travel_time_seconds,
    reachable,
    status,
    error_message,
    calculated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9::timestamptz, now()))
ON CONFLICT (source_waste_source_id, target_transfer_station_id, costing_model)
WHERE source_waste_source_id IS NOT NULL
  AND target_transfer_station_id IS NOT NULL
DO UPDATE SET
    distance_km = EXCLUDED.distance_km,
    travel_time_seconds = EXCLUDED.travel_time_seconds,
    reachable = EXCLUDED.reachable,
    status = EXCLUDED.status,
    error_message = EXCLUDED.error_message,
    calculated_at = EXCLUDED.calculated_at`

const upsertSourceToPlantRouteSQL = `
INSERT INTO waste_routing.route_costs (
    source_waste_source_id,
    target_plant_id,
    costing_model,
    distance_km,
    travel_time_seconds,
    reachable,
    status,
    error_message,
    calculated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9::timestamptz, now()))
ON CONFLICT (source_waste_source_id, target_plant_id, costing_model)
WHERE source_waste_source_id IS NOT NULL
  AND target_plant_id IS NOT NULL
DO UPDATE SET
    distance_km = EXCLUDED.distance_km,
    travel_time_seconds = EXCLUDED.travel_time_seconds,
    reachable = EXCLUDED.reachable,
    status = EXCLUDED.status,
    error_message = EXCLUDED.error_message,
    calculated_at = EXCLUDED.calculated_at`

const upsertTransferToPlantRouteSQL = `
INSERT INTO waste_routing.route_costs (
    source_transfer_station_id,
    target_plant_id,
    costing_model,
    distance_km,
    travel_time_seconds,
    reachable,
    status,
    error_message,
    calculated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9::timestamptz, now()))
ON CONFLICT (source_transfer_station_id, target_plant_id, costing_model)
WHERE source_transfer_station_id IS NOT NULL
  AND target_plant_id IS NOT NULL
DO UPDATE SET
    distance_km = EXCLUDED.distance_km,
    travel_time_seconds = EXCLUDED.travel_time_seconds,
    reachable = EXCLUDED.reachable,
    status = EXCLUDED.status,
    error_message = EXCLUDED.error_message,
    calculated_at = EXCLUDED.calculated_at`

type routeDirectionQueries struct {
	upsert string
	get    string
	list   string
}

// UpsertRouteCost inserts or replaces a Valhalla result for the same endpoint
// pair and costing model.
func (repository *Repository) UpsertRouteCost(ctx context.Context, route models.RouteCost) error {
	statement, err := routeCostStatement(route)
	if err != nil {
		return err
	}
	if _, err := repository.pool.Exec(ctx, statement.query, statement.args...); err != nil {
		return fmt.Errorf(
			"upsert route cost %s %q to %s %q: %w",
			route.SourceType,
			route.SourceID,
			route.TargetType,
			route.TargetID,
			err,
		)
	}
	return nil
}

// UpsertRouteCosts writes all route results atomically in bounded batches.
func (repository *Repository) UpsertRouteCosts(ctx context.Context, routes []models.RouteCost) error {
	return repository.executeBatchTransaction(ctx, "upsert route costs", len(routes), func(index int) (batchStatement, error) {
		return routeCostStatement(routes[index])
	})
}

// GetRouteCost returns one route result by endpoint pair and costing model.
func (repository *Repository) GetRouteCost(
	ctx context.Context,
	sourceType models.ObjectType,
	sourceID string,
	targetType models.ObjectType,
	targetID string,
	costingModel string,
) (models.RouteCost, error) {
	queries, err := queriesForRouteDirection(sourceType, targetType)
	if err != nil {
		return models.RouteCost{}, err
	}
	if strings.TrimSpace(sourceID) == "" || strings.TrimSpace(targetID) == "" {
		return models.RouteCost{}, fmt.Errorf("%w: route endpoint ID is blank", ErrInvalidRoute)
	}
	costingModel = normalizedCostingModel(costingModel)

	route, err := scanRouteCost(repository.pool.QueryRow(ctx, queries.get, sourceID, targetID, costingModel))
	if err != nil {
		operation := fmt.Sprintf(
			"get route cost %s %q to %s %q",
			sourceType,
			sourceID,
			targetType,
			targetID,
		)
		return models.RouteCost{}, mapRowError(operation, err)
	}
	return route, nil
}

// ListRouteCosts returns all routes for one allowed direction and costing model.
func (repository *Repository) ListRouteCosts(
	ctx context.Context,
	sourceType models.ObjectType,
	targetType models.ObjectType,
	costingModel string,
) ([]models.RouteCost, error) {
	queries, err := queriesForRouteDirection(sourceType, targetType)
	if err != nil {
		return nil, err
	}
	costingModel = normalizedCostingModel(costingModel)

	rows, err := repository.pool.Query(ctx, queries.list, costingModel)
	if err != nil {
		return nil, fmt.Errorf("list route costs %s to %s: query: %w", sourceType, targetType, err)
	}
	defer rows.Close()

	routes := make([]models.RouteCost, 0)
	for rows.Next() {
		route, err := scanRouteCost(rows)
		if err != nil {
			return nil, fmt.Errorf("list route costs %s to %s: scan row: %w", sourceType, targetType, err)
		}
		routes = append(routes, route)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list route costs %s to %s: iterate rows: %w", sourceType, targetType, err)
	}
	return routes, nil
}

func routeCostStatement(route models.RouteCost) (batchStatement, error) {
	queries, err := queriesForRouteDirection(route.SourceType, route.TargetType)
	if err != nil {
		return batchStatement{}, err
	}
	if strings.TrimSpace(route.SourceID) == "" || strings.TrimSpace(route.TargetID) == "" {
		return batchStatement{}, fmt.Errorf("%w: route endpoint ID is blank", ErrInvalidRoute)
	}
	if err := validateRouteResult(route); err != nil {
		return batchStatement{}, err
	}

	var calculatedAt any
	if !route.CalculatedAt.IsZero() {
		calculatedAt = route.CalculatedAt
	}
	return batchStatement{
		query: queries.upsert,
		args: []any{
			route.SourceID,
			route.TargetID,
			normalizedCostingModel(route.CostingModel),
			route.DistanceKM,
			route.TravelTimeSeconds,
			route.Reachable,
			route.Status,
			route.ErrorMessage,
			calculatedAt,
		},
	}, nil
}

func queriesForRouteDirection(sourceType models.ObjectType, targetType models.ObjectType) (routeDirectionQueries, error) {
	switch {
	case sourceType == models.ObjectTypeWasteSource && targetType == models.ObjectTypeTransferStation:
		return routeDirectionQueries{
			upsert: upsertSourceToTransferRouteSQL,
			get: routeCostSelectColumns + `
WHERE source_waste_source_id = $1
  AND target_transfer_station_id = $2
  AND costing_model = $3`,
			list: routeCostSelectColumns + `
WHERE source_waste_source_id IS NOT NULL
  AND target_transfer_station_id IS NOT NULL
  AND costing_model = $1
ORDER BY source_waste_source_id, target_transfer_station_id`,
		}, nil
	case sourceType == models.ObjectTypeWasteSource && targetType == models.ObjectTypePlant:
		return routeDirectionQueries{
			upsert: upsertSourceToPlantRouteSQL,
			get: routeCostSelectColumns + `
WHERE source_waste_source_id = $1
  AND target_plant_id = $2
  AND costing_model = $3`,
			list: routeCostSelectColumns + `
WHERE source_waste_source_id IS NOT NULL
  AND target_plant_id IS NOT NULL
  AND costing_model = $1
ORDER BY source_waste_source_id, target_plant_id`,
		}, nil
	case sourceType == models.ObjectTypeTransferStation && targetType == models.ObjectTypePlant:
		return routeDirectionQueries{
			upsert: upsertTransferToPlantRouteSQL,
			get: routeCostSelectColumns + `
WHERE source_transfer_station_id = $1
  AND target_plant_id = $2
  AND costing_model = $3`,
			list: routeCostSelectColumns + `
WHERE source_transfer_station_id IS NOT NULL
  AND target_plant_id IS NOT NULL
  AND costing_model = $1
ORDER BY source_transfer_station_id, target_plant_id`,
		}, nil
	default:
		return routeDirectionQueries{}, fmt.Errorf(
			"%w: unsupported direction %q to %q",
			ErrInvalidRoute,
			sourceType,
			targetType,
		)
	}
}

func validateRouteResult(route models.RouteCost) error {
	if route.Reachable {
		if route.Status != models.RouteCostStatusOK {
			return fmt.Errorf("%w: reachable route must have status %q", ErrInvalidRoute, models.RouteCostStatusOK)
		}
		if !validNonnegativeFloat(route.DistanceKM) || !validNonnegativeFloat(route.TravelTimeSeconds) {
			return fmt.Errorf("%w: reachable route requires finite nonnegative distance and travel time", ErrInvalidRoute)
		}
		return nil
	}

	if route.DistanceKM != nil || route.TravelTimeSeconds != nil {
		return fmt.Errorf("%w: unreachable route cannot have distance or travel time", ErrInvalidRoute)
	}
	switch route.Status {
	case models.RouteCostStatusUnreachable:
		return nil
	case models.RouteCostStatusError:
		if route.ErrorMessage == nil || strings.TrimSpace(*route.ErrorMessage) == "" {
			return fmt.Errorf("%w: error route requires a message", ErrInvalidRoute)
		}
		return nil
	default:
		return fmt.Errorf("%w: unreachable route has unsupported status %q", ErrInvalidRoute, route.Status)
	}
}

func validNonnegativeFloat(value *float64) bool {
	return value != nil && *value >= 0 && !math.IsNaN(*value) && !math.IsInf(*value, 0)
}

func normalizedCostingModel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "auto"
	}
	return value
}

func scanRouteCost(scanner rowScanner) (models.RouteCost, error) {
	var route models.RouteCost
	var sourceType string
	var targetType string
	var status string
	var distance pgtype.Float8
	var travelTime pgtype.Float8
	var errorMessage pgtype.Text
	if err := scanner.Scan(
		&route.ID,
		&sourceType,
		&route.SourceID,
		&targetType,
		&route.TargetID,
		&route.CostingModel,
		&distance,
		&travelTime,
		&route.Reachable,
		&status,
		&errorMessage,
		&route.CalculatedAt,
	); err != nil {
		return models.RouteCost{}, err
	}

	route.SourceType = models.ObjectType(sourceType)
	route.TargetType = models.ObjectType(targetType)
	route.Status = models.RouteCostStatus(status)
	route.DistanceKM = nullableFloatPointer(distance)
	route.TravelTimeSeconds = nullableFloatPointer(travelTime)
	if errorMessage.Valid {
		message := errorMessage.String
		route.ErrorMessage = &message
	}
	return route, nil
}

func nullableFloatPointer(value pgtype.Float8) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
}
