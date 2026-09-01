package qgis

import (
	"fmt"
	"path/filepath"
	"strings"

	"waste-routing/internal/models"
	"waste-routing/internal/utils"
)

// Config describes prepared input CSV files and the QGIS output directory.
type Config struct {
	WasteSourcesPath      string
	TransferStationsPath  string
	PlantsPath            string
	SourceAssignmentsPath string
	TransferFlowsPath     string
	OutputDir             string
}

// OutputPaths contains generated QGIS layer paths.
type OutputPaths struct {
	WasteSources              string
	TransferStations          string
	Plants                    string
	SourceAssignments         string
	TransferToPlantFlows      string
	WasteSourcesStyle         string
	TransferStationsStyle     string
	PlantsStyle               string
	SourceAssignmentsStyle    string
	TransferToPlantFlowsStyle string
}

// ExportLayers writes all QGIS GeoJSON layers for the current project stage.
func ExportLayers(config Config) (OutputPaths, error) {
	outputDir := outputDirectory(config.OutputDir)

	layers, err := readPreparedPointLayers(config)
	if err != nil {
		return OutputPaths{}, err
	}

	paths := makeOutputPaths(outputDir)
	styleCatalog := buildStyleCatalog(layers.transferStations, layers.plants)
	applyFacilityPointStyles(layers.transferStations, models.ObjectTypeTransferStation, styleCatalog)
	applyFacilityPointStyles(layers.plants, models.ObjectTypePlant, styleCatalog)

	sourceIndex, err := indexPointFeatures(layers.wasteSources, "waste sources")
	if err != nil {
		return OutputPaths{}, err
	}
	transferIndex, err := indexPointFeatures(layers.transferStations, "transfer stations")
	if err != nil {
		return OutputPaths{}, err
	}
	plantIndex, err := indexPointFeatures(layers.plants, "plants")
	if err != nil {
		return OutputPaths{}, err
	}

	assignments, err := readSourceAssignmentLines(config.SourceAssignmentsPath, sourceIndex, transferIndex, plantIndex)
	if err != nil {
		return OutputPaths{}, fmt.Errorf("read source assignments: %w", err)
	}

	flows, err := readTransferToPlantFlowLines(config.TransferFlowsPath, transferIndex, plantIndex)
	if err != nil {
		return OutputPaths{}, fmt.Errorf("read transfer flows: %w", err)
	}

	applyWasteSourceAssignmentStyles(layers.wasteSources, sourceAssignmentIndexFromLines(assignments), styleCatalog)
	applySourceAssignmentLineStyles(assignments, styleCatalog)
	applyTransferFlowLineStyles(flows, styleCatalog)

	if err := writePreparedPointLayers(paths, layers); err != nil {
		return OutputPaths{}, err
	}
	if err := WriteLineLayer(paths.SourceAssignments, assignments); err != nil {
		return OutputPaths{}, fmt.Errorf("write source assignments: %w", err)
	}
	if err := WriteLineLayer(paths.TransferToPlantFlows, flows); err != nil {
		return OutputPaths{}, fmt.Errorf("write transfer flows: %w", err)
	}
	if err := writeQGISStyles(paths, styleCatalog); err != nil {
		return OutputPaths{}, err
	}

	return paths, nil
}

// ExportPreparedPointLayers reads prepared CSV files and writes point GeoJSON layers.
func ExportPreparedPointLayers(config Config) (OutputPaths, error) {
	outputDir := outputDirectory(config.OutputDir)

	layers, err := readPreparedPointLayers(config)
	if err != nil {
		return OutputPaths{}, err
	}

	paths := makeOutputPaths(outputDir)
	styleCatalog := buildStyleCatalog(layers.transferStations, layers.plants)
	applyWasteSourceAssignmentStyles(layers.wasteSources, nil, styleCatalog)
	applyFacilityPointStyles(layers.transferStations, models.ObjectTypeTransferStation, styleCatalog)
	applyFacilityPointStyles(layers.plants, models.ObjectTypePlant, styleCatalog)

	if err := writePreparedPointLayers(paths, layers); err != nil {
		return OutputPaths{}, err
	}
	if err := writeQGISStyles(paths, styleCatalog); err != nil {
		return OutputPaths{}, err
	}

	return paths, nil
}

// WritePointLayer writes one GeoJSON FeatureCollection with point geometry.
func WritePointLayer(path string, points []PointFeature) error {
	collection, err := NewPointFeatureCollection(points)
	if err != nil {
		return err
	}
	return utils.WriteJSON(path, collection)
}

// WriteLineLayer writes one GeoJSON FeatureCollection with LineString geometry.
func WriteLineLayer(path string, lines []LineFeature) error {
	collection, err := NewLineFeatureCollection(lines)
	if err != nil {
		return err
	}
	return utils.WriteJSON(path, collection)
}

type preparedPointLayers struct {
	wasteSources     []PointFeature
	transferStations []PointFeature
	plants           []PointFeature
}

func readPreparedPointLayers(config Config) (preparedPointLayers, error) {
	sources, err := readPointLayer(config.WasteSourcesPath, models.ObjectTypeWasteSource, []string{
		"container_volume_m3",
		"container_count",
		"waste_amount",
		"pickups_warm_period",
		"pickups_cold_period",
	})
	if err != nil {
		return preparedPointLayers{}, fmt.Errorf("read waste sources: %w", err)
	}

	transfers, err := readPointLayer(config.TransferStationsPath, models.ObjectTypeTransferStation, []string{"capacity"})
	if err != nil {
		return preparedPointLayers{}, fmt.Errorf("read transfer stations: %w", err)
	}

	plants, err := readPointLayer(config.PlantsPath, models.ObjectTypePlant, []string{"capacity"})
	if err != nil {
		return preparedPointLayers{}, fmt.Errorf("read plants: %w", err)
	}

	return preparedPointLayers{
		wasteSources:     sources,
		transferStations: transfers,
		plants:           plants,
	}, nil
}

func makeOutputPaths(outputDir string) OutputPaths {
	return OutputPaths{
		WasteSources:              filepath.Join(outputDir, "waste_sources.geojson"),
		TransferStations:          filepath.Join(outputDir, "transfer_stations.geojson"),
		Plants:                    filepath.Join(outputDir, "plants.geojson"),
		SourceAssignments:         filepath.Join(outputDir, "source_assignments.geojson"),
		TransferToPlantFlows:      filepath.Join(outputDir, "transfer_to_plant_flows.geojson"),
		WasteSourcesStyle:         filepath.Join(outputDir, "waste_sources.qml"),
		TransferStationsStyle:     filepath.Join(outputDir, "transfer_stations.qml"),
		PlantsStyle:               filepath.Join(outputDir, "plants.qml"),
		SourceAssignmentsStyle:    filepath.Join(outputDir, "source_assignments.qml"),
		TransferToPlantFlowsStyle: filepath.Join(outputDir, "transfer_to_plant_flows.qml"),
	}
}

func writePreparedPointLayers(paths OutputPaths, layers preparedPointLayers) error {
	if err := WritePointLayer(paths.WasteSources, layers.wasteSources); err != nil {
		return fmt.Errorf("write waste sources: %w", err)
	}
	if err := WritePointLayer(paths.TransferStations, layers.transferStations); err != nil {
		return fmt.Errorf("write transfer stations: %w", err)
	}
	if err := WritePointLayer(paths.Plants, layers.plants); err != nil {
		return fmt.Errorf("write plants: %w", err)
	}
	return nil
}

func outputDirectory(outputDir string) string {
	if strings.TrimSpace(outputDir) == "" {
		return filepath.FromSlash("output/qgis")
	}
	return outputDir
}

func readPointLayer(path string, objectType models.ObjectType, numericColumns []string) ([]PointFeature, error) {
	rows, err := utils.ReadCSVRows(path, []string{"id", "latitude", "longitude"})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s: no records found", path)
	}

	points := make([]PointFeature, 0, len(rows))
	for _, row := range rows {
		point, err := rowToPointFeature(row, objectType, numericColumns)
		if err != nil {
			return nil, err
		}
		points = append(points, point)
	}

	return points, nil
}

func rowToPointFeature(row utils.CSVRow, objectType models.ObjectType, numericColumns []string) (PointFeature, error) {
	id, err := row.RequiredValue("id")
	if err != nil {
		return PointFeature{}, err
	}

	latitude, err := row.RequiredFloat("latitude")
	if err != nil {
		return PointFeature{}, fmt.Errorf("invalid latitude: %w", err)
	}
	longitude, err := row.RequiredFloat("longitude")
	if err != nil {
		return PointFeature{}, fmt.Errorf("invalid longitude: %w", err)
	}

	properties := make(map[string]any, len(row.Values)+1)
	for key, value := range row.Values {
		properties[key] = value
	}
	properties["id"] = id
	properties["object_type"] = string(objectType)
	properties["latitude"] = latitude
	properties["longitude"] = longitude

	for _, column := range numericColumns {
		value := strings.TrimSpace(row.Values[column])
		if value == "" {
			continue
		}
		parsed, err := utils.ParseFlexibleFloat(value)
		if err != nil {
			return PointFeature{}, fmt.Errorf("%s:%d: invalid %s: %w", row.Path, row.Line, column, err)
		}
		properties[column] = parsed
	}

	return PointFeature{
		Latitude:   latitude,
		Longitude:  longitude,
		Properties: properties,
	}, nil
}
