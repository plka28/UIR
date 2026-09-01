package input

import (
	"path/filepath"
	"strconv"

	"waste-routing/internal/models"
	"waste-routing/internal/utils"
)

// PreparedData contains normalized records ready for later project stages.
type PreparedData struct {
	WasteSources     []models.WasteSource     `json:"waste_sources"`
	TransferStations []models.TransferStation `json:"transfer_stations"`
	Plants           []models.Plant           `json:"plants"`
	Points           []models.Point           `json:"points"`
	Report           ValidationReport         `json:"report"`
}

// BuildPoints flattens source, transfer station and plant coordinates into one point list.
func BuildPoints(sources []models.WasteSource, stations []models.TransferStation, plants []models.Plant) []models.Point {
	points := make([]models.Point, 0, len(sources)+len(stations)+len(plants))

	for _, source := range sources {
		points = append(points, models.Point{
			ID:        source.ID,
			Type:      models.ObjectTypeWasteSource,
			Name:      source.ID,
			Address:   source.Address,
			Latitude:  source.Latitude,
			Longitude: source.Longitude,
		})
	}
	for _, station := range stations {
		points = append(points, models.Point{
			ID:        station.ID,
			Type:      models.ObjectTypeTransferStation,
			Name:      station.Name,
			Latitude:  station.Latitude,
			Longitude: station.Longitude,
		})
	}
	for _, plant := range plants {
		points = append(points, models.Point{
			ID:        plant.ID,
			Type:      models.ObjectTypePlant,
			Name:      plant.Name,
			Latitude:  plant.Latitude,
			Longitude: plant.Longitude,
		})
	}

	return points
}

// WritePreparedData writes CSV/JSON outputs required by the technical assignment.
func WritePreparedData(outputDir string, data PreparedData) error {
	if err := writeWasteSourcesCSV(filepath.Join(outputDir, "waste_sources.csv"), data.WasteSources); err != nil {
		return err
	}
	if err := writeTransferStationsCSV(filepath.Join(outputDir, "transfer_stations.csv"), data.TransferStations); err != nil {
		return err
	}
	if err := writePlantsCSV(filepath.Join(outputDir, "plants.csv"), data.Plants); err != nil {
		return err
	}
	if err := utils.WriteJSON(filepath.Join(outputDir, "points.json"), data.Points); err != nil {
		return err
	}
	return utils.WriteJSON(filepath.Join(outputDir, "validation_report.json"), data.Report)
}

func writeWasteSourcesCSV(path string, items []models.WasteSource) error {
	header := []string{
		"id",
		"service_zone",
		"address",
		"district",
		"waste_type",
		"container_type",
		"container_volume_m3",
		"container_count",
		"waste_amount",
		"pickups_warm_period",
		"pickups_cold_period",
		"latitude",
		"longitude",
	}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.ID,
			item.ServiceZone,
			item.Address,
			item.District,
			item.WasteType,
			item.ContainerType,
			formatFloat(item.ContainerVolumeM3),
			strconv.Itoa(item.ContainerCount),
			formatFloat(item.WasteAmount),
			formatFloat(item.PickupsWarmPeriod),
			formatFloat(item.PickupsColdPeriod),
			formatFloat(item.Latitude),
			formatFloat(item.Longitude),
		})
	}
	return utils.WriteCSV(path, header, rows)
}

func writeTransferStationsCSV(path string, items []models.TransferStation) error {
	header := []string{"id", "name", "capacity", "latitude", "longitude"}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.ID,
			item.Name,
			formatFloat(item.Capacity),
			formatFloat(item.Latitude),
			formatFloat(item.Longitude),
		})
	}
	return utils.WriteCSV(path, header, rows)
}

func writePlantsCSV(path string, items []models.Plant) error {
	header := []string{"id", "name", "capacity", "latitude", "longitude"}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.ID,
			item.Name,
			formatFloat(item.Capacity),
			formatFloat(item.Latitude),
			formatFloat(item.Longitude),
		})
	}
	return utils.WriteCSV(path, header, rows)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
