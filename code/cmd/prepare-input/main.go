package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"waste-routing/internal/input"
)

func main() {
	config := parseConfig()

	data, err := input.Prepare(config)
	if err != nil {
		log.Fatalf("prepare input: %v", err)
	}

	fmt.Printf("prepared waste sources: %d\n", len(data.WasteSources))
	fmt.Printf("prepared transfer stations: %d\n", len(data.TransferStations))
	fmt.Printf("prepared plants: %d\n", len(data.Plants))
	fmt.Printf("validation issues: %d\n", len(data.Report.Issues))
	fmt.Printf("output directory: %s\n", config.OutputDir)
}

func parseConfig() input.Config {
	defaultSources := filepath.FromSlash("../Данные_Сергиево-Посадская/Сергиево-Посадская_площадки.xlsx")
	defaultFacilities := filepath.FromSlash("../Данные_Сергиево-Посадская/МПС.xlsx")

	config := input.Config{}
	flag.StringVar(&config.SourcesFile, "sources", defaultSources, "path to waste sources XLSX file")
	flag.StringVar(&config.SourcesSheet, "sources-sheet", "Sheet1", "waste sources sheet name")
	flag.StringVar(&config.TransfersFile, "transfers", defaultFacilities, "path to transfer stations XLSX file")
	flag.StringVar(&config.TransfersSheet, "transfers-sheet", "МПС", "transfer stations sheet name")
	flag.StringVar(&config.PlantsFile, "plants", defaultFacilities, "path to plants XLSX file")
	flag.StringVar(&config.PlantsSheet, "plants-sheet", "КПО", "plants sheet name")
	flag.StringVar(&config.OutputDir, "out", filepath.FromSlash("data/prepared"), "output directory for prepared CSV/JSON files")
	flag.Parse()

	return config
}
