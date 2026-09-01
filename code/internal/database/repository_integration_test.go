package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"waste-routing/internal/models"
)

const integrationTestEnv = "WASTE_ROUTING_DATABASE_INTEGRATION"

func TestRepositoryIntegrationRoundTrip(t *testing.T) {
	repository := openIntegrationRepository(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prefix := fmt.Sprintf("repository-test-%d", time.Now().UnixNano())
	sourceID := prefix + "-source"
	stationID := prefix + "-station"
	plantID := prefix + "-plant"
	description := prefix + "-run"
	registerIntegrationCleanup(t, repository, sourceID, stationID, plantID, description)

	source := models.WasteSource{
		ID:                sourceID,
		ServiceZone:       "test-zone",
		Address:           "O'Brien'); DROP TABLE waste_routing.plants; --",
		District:          "test-district",
		WasteType:         "test-waste",
		ContainerType:     "test-container",
		ContainerVolumeM3: 1.1,
		ContainerCount:    2,
		WasteAmount:       12.5,
		PickupsWarmPeriod: math.NaN(),
		PickupsColdPeriod: 4,
		Latitude:          55.751244,
		Longitude:         37.618423,
	}
	station := models.TransferStation{
		ID:        stationID,
		Name:      "Integration station",
		Capacity:  1000,
		Latitude:  55.81,
		Longitude: 37.71,
	}
	plant := models.Plant{
		ID:        plantID,
		Name:      "Integration plant",
		Capacity:  2000,
		Latitude:  55.92,
		Longitude: 37.82,
	}

	if err := repository.UpsertWasteSources(ctx, []models.WasteSource{source}); err != nil {
		t.Fatalf("UpsertWasteSources: %v", err)
	}
	if err := repository.UpsertTransferStations(ctx, []models.TransferStation{station}); err != nil {
		t.Fatalf("UpsertTransferStations: %v", err)
	}
	if err := repository.UpsertPlants(ctx, []models.Plant{plant}); err != nil {
		t.Fatalf("UpsertPlants: %v", err)
	}

	storedSource, err := repository.GetWasteSource(ctx, sourceID)
	if err != nil {
		t.Fatalf("GetWasteSource: %v", err)
	}
	if storedSource.Address != source.Address || !math.IsNaN(storedSource.PickupsWarmPeriod) {
		t.Fatalf("waste source did not round-trip: %#v", storedSource)
	}
	if storedSource.Latitude != source.Latitude || storedSource.Longitude != source.Longitude {
		t.Fatalf("coordinates were swapped: got lat=%v lon=%v", storedSource.Latitude, storedSource.Longitude)
	}

	station.Name = "Updated integration station"
	if err := repository.UpsertTransferStation(ctx, station); err != nil {
		t.Fatalf("UpsertTransferStation update: %v", err)
	}
	storedStation, err := repository.GetTransferStation(ctx, stationID)
	if err != nil || storedStation.Name != station.Name {
		t.Fatalf("GetTransferStation after update: station=%#v err=%v", storedStation, err)
	}
	if _, err := repository.GetPlant(ctx, plantID); err != nil {
		t.Fatalf("GetPlant: %v", err)
	}

	distanceSourceTransfer := 10.0
	timeSourceTransfer := 600.0
	distanceTransferPlant := 15.0
	timeTransferPlant := 900.0
	routeError := "no route"
	costingModel := prefix + "-auto"
	routes := []models.RouteCost{
		{
			SourceType:        models.ObjectTypeWasteSource,
			SourceID:          sourceID,
			TargetType:        models.ObjectTypeTransferStation,
			TargetID:          stationID,
			CostingModel:      costingModel,
			DistanceKM:        &distanceSourceTransfer,
			TravelTimeSeconds: &timeSourceTransfer,
			Reachable:         true,
			Status:            models.RouteCostStatusOK,
		},
		{
			SourceType:   models.ObjectTypeWasteSource,
			SourceID:     sourceID,
			TargetType:   models.ObjectTypePlant,
			TargetID:     plantID,
			CostingModel: costingModel,
			Reachable:    false,
			Status:       models.RouteCostStatusUnreachable,
		},
		{
			SourceType:        models.ObjectTypeTransferStation,
			SourceID:          stationID,
			TargetType:        models.ObjectTypePlant,
			TargetID:          plantID,
			CostingModel:      costingModel,
			DistanceKM:        &distanceTransferPlant,
			TravelTimeSeconds: &timeTransferPlant,
			Reachable:         true,
			Status:            models.RouteCostStatusOK,
			ErrorMessage:      &routeError,
		},
	}
	if err := repository.UpsertRouteCosts(ctx, routes); err != nil {
		t.Fatalf("UpsertRouteCosts: %v", err)
	}
	storedRoute, err := repository.GetRouteCost(
		ctx,
		models.ObjectTypeWasteSource,
		sourceID,
		models.ObjectTypeTransferStation,
		stationID,
		costingModel,
	)
	if err != nil {
		t.Fatalf("GetRouteCost: %v", err)
	}
	if storedRoute.ID == 0 || storedRoute.DistanceKM == nil || *storedRoute.DistanceKM != distanceSourceTransfer {
		t.Fatalf("route cost did not round-trip: %#v", storedRoute)
	}

	run, err := repository.CreateCalculationRun(ctx, models.CalculationRun{
		Description: &description,
		Status:      models.CalculationRunStatusPending,
		Parameters:  json.RawMessage(`{"integration":true}`),
	})
	if err != nil {
		t.Fatalf("CreateCalculationRun: %v", err)
	}
	if run.RunID == 0 || run.Status != models.CalculationRunStatusPending {
		t.Fatalf("unexpected created run: %#v", run)
	}

	assignment := models.SourceAssignment{
		SourceID:        sourceID,
		DestinationType: models.AssignmentToTransferStation,
		DestinationID:   stationID,
		WasteAmount:     12.5,
		DistanceKM:      distanceSourceTransfer,
	}
	flow := models.TransferFlow{
		TransferStationID: stationID,
		PlantID:           plantID,
		WasteAmount:       12.5,
		DistanceKM:        distanceTransferPlant,
	}
	if err := repository.UpsertSourceAssignments(ctx, run.RunID, []models.SourceAssignment{assignment}); err != nil {
		t.Fatalf("UpsertSourceAssignments: %v", err)
	}
	if err := repository.UpsertTransferFlows(ctx, run.RunID, []models.TransferFlow{flow}); err != nil {
		t.Fatalf("UpsertTransferFlows: %v", err)
	}

	assignments, err := repository.ListSourceAssignments(ctx, run.RunID)
	if err != nil || len(assignments) != 1 {
		t.Fatalf("ListSourceAssignments: assignments=%#v err=%v", assignments, err)
	}
	if assignments[0].TransportCost != assignment.WasteAmount*assignment.DistanceKM {
		t.Fatalf("unexpected generated assignment cost: %v", assignments[0].TransportCost)
	}
	flows, err := repository.ListTransferFlows(ctx, run.RunID)
	if err != nil || len(flows) != 1 {
		t.Fatalf("ListTransferFlows: flows=%#v err=%v", flows, err)
	}
	if flows[0].TransportCost != flow.WasteAmount*flow.DistanceKM {
		t.Fatalf("unexpected generated flow cost: %v", flows[0].TransportCost)
	}

	startedAt := run.CreatedAt.Add(time.Second)
	finishedAt := startedAt.Add(time.Second)
	objectiveValue := assignments[0].TransportCost + flows[0].TransportCost
	solverExitFlag := 1
	solveTime := 0.25
	run.Status = models.CalculationRunStatusSucceeded
	run.StartedAt = &startedAt
	run.FinishedAt = &finishedAt
	run.ObjectiveValue = &objectiveValue
	run.SolverExitFlag = &solverExitFlag
	run.SolveTimeSeconds = &solveTime
	run.SolverMetadata = json.RawMessage(`{"message":"ok"}`)
	if err := repository.UpdateCalculationRun(ctx, run); err != nil {
		t.Fatalf("UpdateCalculationRun: %v", err)
	}
	storedRun, err := repository.GetCalculationRun(ctx, run.RunID)
	if err != nil || storedRun.Status != models.CalculationRunStatusSucceeded {
		t.Fatalf("GetCalculationRun after update: run=%#v err=%v", storedRun, err)
	}

	if _, err := repository.GetPlant(ctx, prefix+"-missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	canceledContext, cancelQuery := context.WithCancel(context.Background())
	cancelQuery()
	if _, err := repository.GetPlant(canceledContext, plantID); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestRepositoryIntegrationBatchRollback(t *testing.T) {
	repository := openIntegrationRepository(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prefix := fmt.Sprintf("repository-rollback-test-%d", time.Now().UnixNano())
	firstID := prefix + "-0000"
	registerWasteSourcePrefixCleanup(t, repository, prefix+"-%")

	valid := models.WasteSource{
		ServiceZone:       "test",
		Address:           "valid address",
		District:          "test",
		WasteType:         "test",
		ContainerType:     "test",
		ContainerVolumeM3: 1,
		ContainerCount:    1,
		WasteAmount:       1,
		PickupsWarmPeriod: 1,
		PickupsColdPeriod: 1,
		Latitude:          55,
		Longitude:         37,
	}
	sources := make([]models.WasteSource, batchSize+1)
	for index := range sources {
		sources[index] = valid
		sources[index].ID = fmt.Sprintf("%s-%04d", prefix, index)
	}
	// The invalid row is in the second internal chunk. Its failure must roll
	// back the first chunk as part of the same transaction.
	sources[len(sources)-1].Address = ""

	err := repository.UpsertWasteSources(ctx, sources)
	if err == nil {
		t.Fatal("expected batch constraint error")
	}
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		t.Fatalf("batch error does not preserve pgconn.PgError: %v", err)
	}
	if _, err := repository.GetWasteSource(ctx, firstID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("valid first item was not rolled back: %v", err)
	}
}

func openIntegrationRepository(t *testing.T) *Repository {
	t.Helper()
	if os.Getenv(integrationTestEnv) != "1" {
		t.Skipf("set %s=1 and POSTGRES_* to run PostgreSQL repository tests", integrationTestEnv)
	}

	config, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repository, err := Open(ctx, config)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(repository.Close)
	return repository
}

func registerIntegrationCleanup(
	t *testing.T,
	repository *Repository,
	sourceID string,
	stationID string,
	plantID string,
	runDescription string,
) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		queries := []struct {
			query string
			args  []any
		}{
			{
				query: `DELETE FROM waste_routing.calculation_runs WHERE description = $1`,
				args:  []any{nullableCleanupValue(runDescription)},
			},
			{
				query: `DELETE FROM waste_routing.route_costs
WHERE source_waste_source_id = $1
   OR source_transfer_station_id = $2
   OR target_transfer_station_id = $2
   OR target_plant_id = $3`,
				args: []any{nullableCleanupValue(sourceID), nullableCleanupValue(stationID), nullableCleanupValue(plantID)},
			},
			{
				query: `DELETE FROM waste_routing.waste_sources WHERE id = $1`,
				args:  []any{nullableCleanupValue(sourceID)},
			},
			{
				query: `DELETE FROM waste_routing.transfer_stations WHERE id = $1`,
				args:  []any{nullableCleanupValue(stationID)},
			},
			{
				query: `DELETE FROM waste_routing.plants WHERE id = $1`,
				args:  []any{nullableCleanupValue(plantID)},
			},
		}
		for _, item := range queries {
			if _, err := repository.pool.Exec(ctx, item.query, item.args...); err != nil {
				t.Errorf("clean integration fixture: %v", err)
			}
		}
	})
}

func nullableCleanupValue(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func registerWasteSourcePrefixCleanup(t *testing.T, repository *Repository, pattern string) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := repository.pool.Exec(
			ctx,
			`DELETE FROM waste_routing.waste_sources WHERE id LIKE $1`,
			pattern,
		); err != nil {
			t.Errorf("clean rollback test fixtures: %v", err)
		}
	})
}
