package input

import (
	"fmt"
	"math"

	"waste-routing/internal/models"
)

// ValidationIssue describes one rejected source row or suspicious input value.
type ValidationIssue struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Field      string `json:"field"`
	Message    string `json:"message"`
}

// ValidationReport summarizes filtering performed during input preparation.
type ValidationReport struct {
	InputWasteSources      int               `json:"input_waste_sources"`
	OutputWasteSources     int               `json:"output_waste_sources"`
	InputTransferStations  int               `json:"input_transfer_stations"`
	OutputTransferStations int               `json:"output_transfer_stations"`
	InputPlants            int               `json:"input_plants"`
	OutputPlants           int               `json:"output_plants"`
	Issues                 []ValidationIssue `json:"issues"`
}

// ValidateWasteSources returns only rows that can be used by later routing steps.
func ValidateWasteSources(items []models.WasteSource) ([]models.WasteSource, []ValidationIssue) {
	valid := make([]models.WasteSource, 0, len(items))
	var issues []ValidationIssue

	for _, item := range items {
		itemIssues := validateWasteSource(item)
		if len(itemIssues) > 0 {
			issues = append(issues, itemIssues...)
			continue
		}
		valid = append(valid, item)
	}

	return valid, issues
}

// ValidateTransferStations returns only stations with usable coordinates and capacity.
func ValidateTransferStations(items []models.TransferStation) ([]models.TransferStation, []ValidationIssue) {
	valid := make([]models.TransferStation, 0, len(items))
	var issues []ValidationIssue

	for _, item := range items {
		itemIssues := validateTransferStation(item)
		if len(itemIssues) > 0 {
			issues = append(issues, itemIssues...)
			continue
		}
		valid = append(valid, item)
	}

	return valid, issues
}

// ValidatePlants returns only plants with usable coordinates and capacity.
func ValidatePlants(items []models.Plant) ([]models.Plant, []ValidationIssue) {
	valid := make([]models.Plant, 0, len(items))
	var issues []ValidationIssue

	for _, item := range items {
		itemIssues := validatePlant(item)
		if len(itemIssues) > 0 {
			issues = append(issues, itemIssues...)
			continue
		}
		valid = append(valid, item)
	}

	return valid, issues
}

func validateWasteSource(item models.WasteSource) []ValidationIssue {
	var issues []ValidationIssue
	issues = appendIfBlank(issues, "waste_source", item.ID, "id", item.ID)
	issues = appendIfBlank(issues, "waste_source", item.ID, "address", item.Address)
	issues = appendIfInvalidCoordinate(issues, "waste_source", item.ID, item.Latitude, item.Longitude)
	if isInvalidPositive(item.ContainerVolumeM3) {
		issues = appendIssue(issues, "waste_source", item.ID, "container_volume_m3", "must be positive")
	}
	if item.ContainerCount <= 0 {
		issues = appendIssue(issues, "waste_source", item.ID, "container_count", "must be positive")
	}
	if isInvalidPositive(item.WasteAmount) {
		issues = appendIssue(issues, "waste_source", item.ID, "waste_amount", "must be positive")
	}
	return issues
}

func validateTransferStation(item models.TransferStation) []ValidationIssue {
	var issues []ValidationIssue
	issues = appendIfBlank(issues, "transfer_station", item.ID, "id", item.ID)
	issues = appendIfBlank(issues, "transfer_station", item.ID, "name", item.Name)
	issues = appendIfInvalidCoordinate(issues, "transfer_station", item.ID, item.Latitude, item.Longitude)
	if isInvalidPositive(item.Capacity) {
		issues = appendIssue(issues, "transfer_station", item.ID, "capacity", "must be positive")
	}
	return issues
}

func validatePlant(item models.Plant) []ValidationIssue {
	var issues []ValidationIssue
	issues = appendIfBlank(issues, "plant", item.ID, "id", item.ID)
	issues = appendIfBlank(issues, "plant", item.ID, "name", item.Name)
	issues = appendIfInvalidCoordinate(issues, "plant", item.ID, item.Latitude, item.Longitude)
	if isInvalidPositive(item.Capacity) {
		issues = appendIssue(issues, "plant", item.ID, "capacity", "must be positive")
	}
	return issues
}

func appendIfBlank(issues []ValidationIssue, entityType string, entityID string, field string, value string) []ValidationIssue {
	if value == "" {
		return appendIssue(issues, entityType, entityID, field, "is required")
	}
	return issues
}

func appendIfInvalidCoordinate(issues []ValidationIssue, entityType string, entityID string, latitude float64, longitude float64) []ValidationIssue {
	if math.IsNaN(latitude) || latitude < -90 || latitude > 90 {
		issues = appendIssue(issues, entityType, entityID, "latitude", fmt.Sprintf("invalid latitude: %v", latitude))
	}
	if math.IsNaN(longitude) || longitude < -180 || longitude > 180 {
		issues = appendIssue(issues, entityType, entityID, "longitude", fmt.Sprintf("invalid longitude: %v", longitude))
	}
	return issues
}

func isInvalidPositive(value float64) bool {
	return math.IsNaN(value) || value <= 0
}

func appendIssue(issues []ValidationIssue, entityType string, entityID string, field string, message string) []ValidationIssue {
	return append(issues, ValidationIssue{
		EntityType: entityType,
		EntityID:   entityID,
		Field:      field,
		Message:    message,
	})
}
