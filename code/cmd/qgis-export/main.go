package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"waste-routing/internal/qgis"
)

func main() {
	config := parseConfig()

	paths, err := qgis.ExportLayers(config)
	if err != nil {
		log.Fatalf("export QGIS layers: %v", err)
	}

	fmt.Printf("saved %s\n", paths.WasteSources)
	fmt.Printf("saved %s\n", paths.TransferStations)
	fmt.Printf("saved %s\n", paths.Plants)
	fmt.Printf("saved %s\n", paths.SourceAssignments)
	fmt.Printf("saved %s\n", paths.TransferToPlantFlows)
	fmt.Printf("saved %s\n", paths.WasteSourcesStyle)
	fmt.Printf("saved %s\n", paths.TransferStationsStyle)
	fmt.Printf("saved %s\n", paths.PlantsStyle)
	fmt.Printf("saved %s\n", paths.SourceAssignmentsStyle)
	fmt.Printf("saved %s\n", paths.TransferToPlantFlowsStyle)
}

func parseConfig() qgis.Config {
	config := qgis.Config{}
	flag.StringVar(&config.WasteSourcesPath, "sources", filepath.FromSlash("data/prepared/waste_sources.csv"), "prepared waste sources CSV")
	flag.StringVar(&config.TransferStationsPath, "transfers", filepath.FromSlash("data/prepared/transfer_stations.csv"), "prepared transfer stations CSV")
	flag.StringVar(&config.PlantsPath, "plants", filepath.FromSlash("data/prepared/plants.csv"), "prepared plants CSV")
	flag.StringVar(&config.SourceAssignmentsPath, "source-assignments", filepath.FromSlash("output/optimization/source_assignments.csv"), "optimization source assignments CSV")
	flag.StringVar(&config.TransferFlowsPath, "transfer-flows", filepath.FromSlash("output/optimization/transfer_flows.csv"), "optimization transfer-to-plant flows CSV")
	flag.StringVar(&config.OutputDir, "out", filepath.FromSlash("output/qgis"), "output directory for QGIS GeoJSON files")
	flag.Parse()

	return config
}
