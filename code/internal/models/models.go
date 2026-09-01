package models

import (
	"encoding/json"
	"time"
)

// ObjectType identifies the role of a point in the waste logistics model.
type ObjectType string

const (
	ObjectTypeWasteSource     ObjectType = "waste_source"
	ObjectTypeTransferStation ObjectType = "transfer_station"
	ObjectTypePlant           ObjectType = "plant"
)

// WasteSource describes a waste accumulation point used as an optimization source.
type WasteSource struct {
	ID                string  `json:"id"`
	ServiceZone       string  `json:"service_zone"`
	Address           string  `json:"address"`
	District          string  `json:"district"`
	WasteType         string  `json:"waste_type"`
	ContainerType     string  `json:"container_type"`
	ContainerVolumeM3 float64 `json:"container_volume_m3"`
	ContainerCount    int     `json:"container_count"`
	WasteAmount       float64 `json:"waste_amount"` // tons per year
	PickupsWarmPeriod float64 `json:"pickups_warm_period"`
	PickupsColdPeriod float64 `json:"pickups_cold_period"`
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
}

// TransferStation describes a waste transfer station with finite capacity.
type TransferStation struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Capacity  float64 `json:"capacity"` // tons per year
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Plant describes a processing or disposal plant with finite capacity.
type Plant struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Capacity  float64 `json:"capacity"` // tons per year
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Point is a normalized geographic point used by routing and visualization steps.
type Point struct {
	ID        string     `json:"id"`
	Type      ObjectType `json:"type"`
	Name      string     `json:"name"`
	Address   string     `json:"address,omitempty"`
	Latitude  float64    `json:"latitude"`
	Longitude float64    `json:"longitude"`
}

// DistanceMatrix stores pairwise routing results between two point groups.
type DistanceMatrix struct {
	FromType     ObjectType  `json:"from_type"`
	ToType       ObjectType  `json:"to_type"`
	FromIDs      []string    `json:"from_ids"`
	ToIDs        []string    `json:"to_ids"`
	DistancesKM  [][]float64 `json:"distances_km"`
	DurationsMin [][]float64 `json:"durations_min"`
}

// RouteCostStatus describes the outcome of a Valhalla route calculation.
type RouteCostStatus string

const (
	RouteCostStatusOK          RouteCostStatus = "ok"
	RouteCostStatusUnreachable RouteCostStatus = "unreachable"
	RouteCostStatusError       RouteCostStatus = "error"
)

// RouteCost stores one routing result between an allowed pair of object types.
// Distance and travel time are nil when the route is unreachable or failed.
type RouteCost struct {
	ID                int64           `json:"id"`
	SourceType        ObjectType      `json:"source_type"`
	SourceID          string          `json:"source_id"`
	TargetType        ObjectType      `json:"target_type"`
	TargetID          string          `json:"target_id"`
	CostingModel      string          `json:"costing_model"`
	DistanceKM        *float64        `json:"distance_km,omitempty"`
	TravelTimeSeconds *float64        `json:"travel_time_seconds,omitempty"`
	Reachable         bool            `json:"reachable"`
	Status            RouteCostStatus `json:"status"`
	ErrorMessage      *string         `json:"error_message,omitempty"`
	CalculatedAt      time.Time       `json:"calculated_at"`
}

// CalculationRunStatus identifies the lifecycle state of an optimization run.
type CalculationRunStatus string

const (
	CalculationRunStatusPending   CalculationRunStatus = "pending"
	CalculationRunStatusRunning   CalculationRunStatus = "running"
	CalculationRunStatusSucceeded CalculationRunStatus = "succeeded"
	CalculationRunStatusFailed    CalculationRunStatus = "failed"
)

// CalculationRun contains parameters, lifecycle data, and the solver summary
// for one optimization run.
type CalculationRun struct {
	RunID            int64                `json:"run_id"`
	Description      *string              `json:"description,omitempty"`
	Status           CalculationRunStatus `json:"status"`
	Parameters       json.RawMessage      `json:"parameters"`
	CreatedAt        time.Time            `json:"created_at"`
	StartedAt        *time.Time           `json:"started_at,omitempty"`
	FinishedAt       *time.Time           `json:"finished_at,omitempty"`
	ObjectiveValue   *float64             `json:"objective_value,omitempty"`
	SolverExitFlag   *int                 `json:"solver_exit_flag,omitempty"`
	SolveTimeSeconds *float64             `json:"solve_time_seconds,omitempty"`
	SolverMetadata   json.RawMessage      `json:"solver_metadata"`
	ErrorMessage     *string              `json:"error_message,omitempty"`
}

// AssignmentDestinationType identifies whether a waste source is assigned directly
// to a plant or through a transfer station.
type AssignmentDestinationType string

const (
	AssignmentToTransferStation AssignmentDestinationType = "transfer_station"
	AssignmentToPlant           AssignmentDestinationType = "plant"
)

// AssignmentResult describes the decoded optimization assignment for one source.
type AssignmentResult struct {
	SourceID        string                    `json:"source_id"`
	DestinationType AssignmentDestinationType `json:"destination_type"`
	DestinationID   string                    `json:"destination_id"`
	WasteAmount     float64                   `json:"waste_amount"`
	DistanceKM      float64                   `json:"distance_km"`
	TransportCost   float64                   `json:"transport_cost"`
}

// SourceAssignment is the database entity name for the existing decoded
// assignment model. RunID is supplied separately by repository methods.
type SourceAssignment = AssignmentResult

// TransferFlow describes an optimized flow from a transfer station to a plant.
type TransferFlow struct {
	TransferStationID string  `json:"transfer_station_id"`
	PlantID           string  `json:"plant_id"`
	WasteAmount       float64 `json:"waste_amount"`
	DistanceKM        float64 `json:"distance_km"`
	TransportCost     float64 `json:"transport_cost"`
}
