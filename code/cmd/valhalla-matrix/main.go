package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"strconv"
	"time"

	"waste-routing/internal/utils"
	"waste-routing/internal/valhalla"
)

type matrixPoint struct {
	ID       string
	Location valhalla.Location
}

func main() {
	config := parseConfig()

	client, err := valhalla.NewClient(valhalla.Config{
		BaseURL: config.baseURL,
		Timeout: time.Duration(config.timeoutSeconds) * time.Second,
	})
	if err != nil {
		log.Fatalf("create Valhalla client: %v", err)
	}

	sources, err := readMatrixPoints(config.sourcesPath)
	if err != nil {
		log.Fatalf("read waste sources: %v", err)
	}
	transfers, err := readMatrixPoints(config.transfersPath)
	if err != nil {
		log.Fatalf("read transfer stations: %v", err)
	}
	plants, err := readMatrixPoints(config.plantsPath)
	if err != nil {
		log.Fatalf("read plants: %v", err)
	}

	jobs := []struct {
		name    string
		from    []matrixPoint
		to      []matrixPoint
		outPath string
	}{
		{
			name:    "source_to_transfer",
			from:    sources,
			to:      transfers,
			outPath: filepath.Join(config.outputDir, "source_to_transfer.csv"),
		},
		{
			name:    "source_to_plant",
			from:    sources,
			to:      plants,
			outPath: filepath.Join(config.outputDir, "source_to_plant.csv"),
		},
		{
			name:    "transfer_to_plant",
			from:    transfers,
			to:      plants,
			outPath: filepath.Join(config.outputDir, "transfer_to_plant.csv"),
		},
	}

	for _, job := range jobs {
		fmt.Printf("calculating %s: %d x %d\n", job.name, len(job.from), len(job.to))
		matrix, err := calculateMatrix(client, job.from, job.to, config)
		if err != nil {
			log.Fatalf("calculate %s: %v", job.name, err)
		}
		if err := writeMatrix(job.outPath, job.from, job.to, matrix); err != nil {
			log.Fatalf("write %s: %v", job.name, err)
		}
		fmt.Printf("saved %s\n", job.outPath)
	}
}

type config struct {
	baseURL        string
	costing        string
	timeoutSeconds int
	batchSize      int
	sourcesPath    string
	transfersPath  string
	plantsPath     string
	outputDir      string
}

func parseConfig() config {
	var config config
	flag.StringVar(&config.baseURL, "base-url", valhalla.DefaultBaseURL, "Valhalla base URL")
	flag.StringVar(&config.costing, "costing", "auto", "Valhalla costing model")
	flag.IntVar(&config.timeoutSeconds, "timeout", 120, "per-request timeout in seconds")
	flag.IntVar(&config.batchSize, "batch-size", 100, "number of source rows per Valhalla matrix request")
	flag.StringVar(&config.sourcesPath, "sources", filepath.FromSlash("data/prepared/waste_sources.csv"), "prepared waste sources CSV")
	flag.StringVar(&config.transfersPath, "transfers", filepath.FromSlash("data/prepared/transfer_stations.csv"), "prepared transfer stations CSV")
	flag.StringVar(&config.plantsPath, "plants", filepath.FromSlash("data/prepared/plants.csv"), "prepared plants CSV")
	flag.StringVar(&config.outputDir, "out", filepath.FromSlash("output/matrices"), "output directory for matrix CSV files")
	flag.Parse()

	if config.batchSize <= 0 {
		log.Fatal("batch-size must be positive")
	}
	if config.timeoutSeconds <= 0 {
		log.Fatal("timeout must be positive")
	}

	return config
}

func calculateMatrix(client *valhalla.Client, sources []matrixPoint, targets []matrixPoint, config config) ([][]*float64, error) {
	matrix := make([][]*float64, len(sources))
	targetLocations := valhallaLocations(targets)

	// Valhalla matrix requests can be large; batching limits source rows per call.
	for start := 0; start < len(sources); start += config.batchSize {
		end := min(start+config.batchSize, len(sources))
		sourceBatch := sources[start:end]

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.timeoutSeconds)*time.Second)
		response, err := client.Matrix(ctx, valhalla.MatrixRequest{
			Sources: valhallaLocations(sourceBatch),
			Targets: targetLocations,
			Costing: config.costing,
			Units:   "kilometers",
		})
		cancel()
		if err != nil {
			return nil, fmt.Errorf("request rows %d-%d: %w", start+1, end, err)
		}
		if err := validateResponseSize(response, len(sourceBatch), len(targets)); err != nil {
			return nil, fmt.Errorf("rows %d-%d: %w", start+1, end, err)
		}

		for i, row := range response.SourcesToTargets {
			matrix[start+i] = make([]*float64, len(targets))
			for j, element := range row {
				matrix[start+i][j] = element.Distance
			}
		}

		fmt.Printf("  processed rows %d-%d of %d\n", start+1, end, len(sources))
	}

	return matrix, nil
}

func readMatrixPoints(path string) ([]matrixPoint, error) {
	rows, err := utils.ReadCSVRows(path, []string{"id", "latitude", "longitude"})
	if err != nil {
		return nil, err
	}

	points := make([]matrixPoint, 0, len(rows))
	for _, row := range rows {
		id, err := row.RequiredValue("id")
		if err != nil {
			return nil, err
		}
		lat, err := parseCoordinate(row, "latitude")
		if err != nil {
			return nil, fmt.Errorf("invalid latitude: %w", err)
		}
		lon, err := parseCoordinate(row, "longitude")
		if err != nil {
			return nil, fmt.Errorf("invalid longitude: %w", err)
		}
		points = append(points, matrixPoint{
			ID:       id,
			Location: valhalla.Location{Lat: lat, Lon: lon},
		})
	}
	if len(points) == 0 {
		return nil, fmt.Errorf("%s: no points found", path)
	}

	return points, nil
}

func parseCoordinate(row utils.CSVRow, column string) (float64, error) {
	parsed, err := row.RequiredFloat(column)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(parsed) {
		return 0, fmt.Errorf("value is NaN")
	}
	return parsed, nil
}

func valhallaLocations(points []matrixPoint) []valhalla.Location {
	result := make([]valhalla.Location, len(points))
	for i, point := range points {
		result[i] = point.Location
	}
	return result
}

func validateResponseSize(response *valhalla.MatrixResponse, sourceCount int, targetCount int) error {
	if len(response.SourcesToTargets) != sourceCount {
		return fmt.Errorf("unexpected row count: got %d, want %d", len(response.SourcesToTargets), sourceCount)
	}
	for i, row := range response.SourcesToTargets {
		if len(row) != targetCount {
			return fmt.Errorf("unexpected column count in row %d: got %d, want %d", i, len(row), targetCount)
		}
	}
	return nil
}

func writeMatrix(path string, sources []matrixPoint, targets []matrixPoint, matrix [][]*float64) error {
	header := make([]string, 0, len(targets)+1)
	header = append(header, "id")
	for _, target := range targets {
		header = append(header, target.ID)
	}

	rows := make([][]string, 0, len(sources))
	for i, source := range sources {
		row := make([]string, 0, len(targets)+1)
		row = append(row, source.ID)
		for _, distance := range matrix[i] {
			row = append(row, formatDistance(distance))
		}
		rows = append(rows, row)
	}

	return utils.WriteCSV(path, header, rows)
}

func formatDistance(distance *float64) string {
	if distance == nil || math.IsNaN(*distance) {
		return "NaN"
	}
	return strconv.FormatFloat(*distance, 'f', -1, 64)
}
