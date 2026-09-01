package database

import (
	"errors"
	"math"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"waste-routing/internal/models"
)

func TestRouteDirections(t *testing.T) {
	tests := []struct {
		name       string
		sourceType models.ObjectType
		targetType models.ObjectType
		wantError  bool
	}{
		{name: "source to transfer", sourceType: models.ObjectTypeWasteSource, targetType: models.ObjectTypeTransferStation},
		{name: "source to plant", sourceType: models.ObjectTypeWasteSource, targetType: models.ObjectTypePlant},
		{name: "transfer to plant", sourceType: models.ObjectTypeTransferStation, targetType: models.ObjectTypePlant},
		{name: "plant to source", sourceType: models.ObjectTypePlant, targetType: models.ObjectTypeWasteSource, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			queries, err := queriesForRouteDirection(test.sourceType, test.targetType)
			if test.wantError {
				if !errors.Is(err, ErrInvalidRoute) {
					t.Fatalf("expected ErrInvalidRoute, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("queriesForRouteDirection: %v", err)
			}
			if queries.upsert == "" || queries.get == "" || queries.list == "" {
				t.Fatal("route direction contains an empty query")
			}
		})
	}
}

func TestValidateRouteResult(t *testing.T) {
	distance := 12.5
	travelTime := 900.0
	valid := models.RouteCost{
		Reachable:         true,
		Status:            models.RouteCostStatusOK,
		DistanceKM:        &distance,
		TravelTimeSeconds: &travelTime,
	}
	if err := validateRouteResult(valid); err != nil {
		t.Fatalf("valid reachable route: %v", err)
	}

	invalid := valid
	invalid.TravelTimeSeconds = nil
	if err := validateRouteResult(invalid); !errors.Is(err, ErrInvalidRoute) {
		t.Fatalf("expected ErrInvalidRoute for missing time, got %v", err)
	}

	errorMessage := "Valhalla failed"
	unreachable := models.RouteCost{
		Reachable:    false,
		Status:       models.RouteCostStatusError,
		ErrorMessage: &errorMessage,
	}
	if err := validateRouteResult(unreachable); err != nil {
		t.Fatalf("valid failed route: %v", err)
	}
}

func TestPickupNullMapping(t *testing.T) {
	if value := nullablePickup(math.NaN()); value != nil {
		t.Fatalf("NaN pickup must become SQL NULL, got %#v", value)
	}
	if value := nullableFloatValue(pgtype.Float8{}); !math.IsNaN(value) {
		t.Fatalf("SQL NULL must become NaN, got %v", value)
	}
	if value := nullableFloatValue(pgtype.Float8{Float64: 3.5, Valid: true}); value != 3.5 {
		t.Fatalf("valid pickup changed: %v", value)
	}
}

func TestSourceAssignmentArguments(t *testing.T) {
	assignment := models.SourceAssignment{
		SourceID:        "source-1",
		DestinationType: models.AssignmentToTransferStation,
		DestinationID:   "transfer-1",
		WasteAmount:     4,
		DistanceKM:      5,
		TransportCost:   999,
	}
	arguments, err := sourceAssignmentArguments(7, assignment)
	if err != nil {
		t.Fatalf("sourceAssignmentArguments: %v", err)
	}
	if arguments[2] != "transfer-1" || arguments[3] != nil {
		t.Fatalf("unexpected typed destination arguments: %#v", arguments)
	}
	if len(arguments) != 6 {
		t.Fatalf("generated transport cost must not be inserted: %#v", arguments)
	}
}
