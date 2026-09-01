package qgis

import (
	"fmt"
	"strings"

	"waste-routing/internal/models"
	"waste-routing/internal/utils"
)

type coordinate struct {
	latitude  float64
	longitude float64
}

func readSourceAssignmentLines(path string, sources map[string]coordinate, transfers map[string]coordinate, plants map[string]coordinate) ([]LineFeature, error) {
	rows, err := utils.ReadCSVRows(path, nil)
	if err != nil {
		return nil, err
	}

	lines := make([]LineFeature, 0, len(rows))
	for _, row := range rows {
		sourceID, err := row.RequiredValue("source_id")
		if err != nil {
			return nil, err
		}
		targetType, err := row.RequiredValue("target_type", "assignment_type", "destination_type")
		if err != nil {
			return nil, err
		}
		targetID, err := row.RequiredValue("target_id", "destination_id")
		if err != nil {
			return nil, err
		}
		wasteVolume, err := row.RequiredFloat("waste_volume", "waste_amount")
		if err != nil {
			return nil, err
		}
		distance, err := row.RequiredFloat("distance", "distance_km")
		if err != nil {
			return nil, err
		}
		cost, err := row.RequiredFloat("cost", "transport_cost")
		if err != nil {
			return nil, err
		}

		source, ok := sources[sourceID]
		if !ok {
			return nil, fmt.Errorf("%s:%d: source_id %q was not found in waste sources", row.Path, row.Line, sourceID)
		}
		target, normalizedTargetType, err := assignmentTargetCoordinate(row, targetType, targetID, transfers, plants)
		if err != nil {
			return nil, err
		}

		lines = append(lines, LineFeature{
			Coordinates: lineCoordinates(source, target),
			Properties: map[string]any{
				"source_id":    sourceID,
				"target_type":  normalizedTargetType,
				"target_id":    targetID,
				"waste_volume": wasteVolume,
				"distance":     distance,
				"cost":         cost,
			},
		})
	}

	return lines, nil
}

func readTransferToPlantFlowLines(path string, transfers map[string]coordinate, plants map[string]coordinate) ([]LineFeature, error) {
	rows, err := utils.ReadCSVRows(path, nil)
	if err != nil {
		return nil, err
	}

	lines := make([]LineFeature, 0, len(rows))
	for _, row := range rows {
		transferID, err := row.RequiredValue("transfer_id", "transfer_station_id")
		if err != nil {
			return nil, err
		}
		plantID, err := row.RequiredValue("plant_id")
		if err != nil {
			return nil, err
		}
		flowVolume, err := row.RequiredFloat("flow_volume", "waste_amount")
		if err != nil {
			return nil, err
		}
		distance, err := row.RequiredFloat("distance", "distance_km")
		if err != nil {
			return nil, err
		}
		cost, err := row.RequiredFloat("cost", "transport_cost")
		if err != nil {
			return nil, err
		}

		transfer, ok := transfers[transferID]
		if !ok {
			return nil, fmt.Errorf("%s:%d: transfer_id %q was not found in transfer stations", row.Path, row.Line, transferID)
		}
		plant, ok := plants[plantID]
		if !ok {
			return nil, fmt.Errorf("%s:%d: plant_id %q was not found in plants", row.Path, row.Line, plantID)
		}

		lines = append(lines, LineFeature{
			Coordinates: lineCoordinates(transfer, plant),
			Properties: map[string]any{
				"transfer_id": transferID,
				"plant_id":    plantID,
				"flow_volume": flowVolume,
				"distance":    distance,
				"cost":        cost,
			},
		})
	}

	return lines, nil
}

func assignmentTargetCoordinate(row utils.CSVRow, targetType string, targetID string, transfers map[string]coordinate, plants map[string]coordinate) (coordinate, string, error) {
	switch normalizeTargetType(targetType) {
	case string(models.ObjectTypeTransferStation):
		target, ok := transfers[targetID]
		if !ok {
			return coordinate{}, "", fmt.Errorf("%s:%d: target transfer station %q was not found", row.Path, row.Line, targetID)
		}
		return target, string(models.ObjectTypeTransferStation), nil
	case string(models.ObjectTypePlant):
		target, ok := plants[targetID]
		if !ok {
			return coordinate{}, "", fmt.Errorf("%s:%d: target plant %q was not found", row.Path, row.Line, targetID)
		}
		return target, string(models.ObjectTypePlant), nil
	default:
		return coordinate{}, "", fmt.Errorf("%s:%d: unsupported target_type %q", row.Path, row.Line, targetType)
	}
}

func normalizeTargetType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	switch value {
	case "transfer", "transfer_station":
		return string(models.ObjectTypeTransferStation)
	case "plant", "direct_plant":
		return string(models.ObjectTypePlant)
	default:
		return value
	}
}

func indexPointFeatures(points []PointFeature, layerName string) (map[string]coordinate, error) {
	index := make(map[string]coordinate, len(points))
	for rowIndex, point := range points {
		id, ok := point.Properties["id"].(string)
		if !ok || strings.TrimSpace(id) == "" {
			return nil, fmt.Errorf("%s point %d has empty id", layerName, rowIndex+1)
		}
		if _, exists := index[id]; exists {
			return nil, fmt.Errorf("%s contains duplicate id %q", layerName, id)
		}
		index[id] = coordinate{
			latitude:  point.Latitude,
			longitude: point.Longitude,
		}
	}
	return index, nil
}

func lineCoordinates(from coordinate, to coordinate) [][2]float64 {
	return [][2]float64{
		{from.longitude, from.latitude},
		{to.longitude, to.latitude},
	}
}
