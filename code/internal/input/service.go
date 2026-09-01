package input

import "fmt"

// Config describes source tables and output location for input preparation.
type Config struct {
	SourcesFile    string
	SourcesSheet   string
	TransfersFile  string
	TransfersSheet string
	PlantsFile     string
	PlantsSheet    string
	OutputDir      string
}

// Prepare reads raw tables, validates records and writes normalized CSV/JSON files.
func Prepare(config Config) (PreparedData, error) {
	sources, err := ReadWasteSources(config.SourcesFile, config.SourcesSheet)
	if err != nil {
		return PreparedData{}, fmt.Errorf("read waste sources: %w", err)
	}

	stations, err := ReadTransferStations(config.TransfersFile, config.TransfersSheet)
	if err != nil {
		return PreparedData{}, fmt.Errorf("read transfer stations: %w", err)
	}

	plants, err := ReadPlants(config.PlantsFile, config.PlantsSheet)
	if err != nil {
		return PreparedData{}, fmt.Errorf("read plants: %w", err)
	}

	validSources, sourceIssues := ValidateWasteSources(sources)
	validStations, stationIssues := ValidateTransferStations(stations)
	validPlants, plantIssues := ValidatePlants(plants)

	issues := make([]ValidationIssue, 0, len(sourceIssues)+len(stationIssues)+len(plantIssues))
	issues = append(issues, sourceIssues...)
	issues = append(issues, stationIssues...)
	issues = append(issues, plantIssues...)

	data := PreparedData{
		WasteSources:     validSources,
		TransferStations: validStations,
		Plants:           validPlants,
		Points:           BuildPoints(validSources, validStations, validPlants),
		Report: ValidationReport{
			InputWasteSources:      len(sources),
			OutputWasteSources:     len(validSources),
			InputTransferStations:  len(stations),
			OutputTransferStations: len(validStations),
			InputPlants:            len(plants),
			OutputPlants:           len(validPlants),
			Issues:                 issues,
		},
	}

	if err := WritePreparedData(config.OutputDir, data); err != nil {
		return PreparedData{}, fmt.Errorf("write prepared data: %w", err)
	}

	return data, nil
}
