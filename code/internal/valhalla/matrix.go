package valhalla

import (
	"context"
	"fmt"
	"math"
)

const sourcesToTargetsEndpoint = "/sources_to_targets"

// Location is a Valhalla matrix source or target coordinate.
type Location struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// MatrixRequest is the JSON body accepted by Valhalla Matrix API.
type MatrixRequest struct {
	Sources []Location `json:"sources"`
	Targets []Location `json:"targets"`
	Costing string     `json:"costing"`
	Units   string     `json:"units,omitempty"`
}

// MatrixResponse is the subset of Valhalla Matrix API response used by this project.
type MatrixResponse struct {
	SourcesToTargets [][]MatrixElement `json:"sources_to_targets"`
	Units            string            `json:"units,omitempty"`
}

// MatrixElement contains distance and time from one source to one target.
type MatrixElement struct {
	Distance  *float64 `json:"distance"`
	Time      *float64 `json:"time"`
	FromIndex int      `json:"from_index"`
	ToIndex   int      `json:"to_index"`
}

// DistanceResult is a convenient single-pair matrix result.
type DistanceResult struct {
	DistanceKM  float64
	TimeSeconds float64
}

// Matrix calls Valhalla /sources_to_targets for the given sources and targets.
func (client *Client) Matrix(ctx context.Context, request MatrixRequest) (*MatrixResponse, error) {
	normalized := normalizeMatrixRequest(request)
	if err := validateMatrixRequest(normalized); err != nil {
		return nil, err
	}

	var response MatrixResponse
	if err := client.postJSON(ctx, sourcesToTargetsEndpoint, normalized, &response); err != nil {
		return nil, err
	}
	if len(response.SourcesToTargets) == 0 {
		return nil, fmt.Errorf("Valhalla response has empty sources_to_targets matrix")
	}

	return &response, nil
}

// Distance calls the matrix endpoint for one source-target pair.
func (client *Client) Distance(ctx context.Context, source Location, target Location, costing string) (DistanceResult, error) {
	response, err := client.Matrix(ctx, MatrixRequest{
		Sources: []Location{source},
		Targets: []Location{target},
		Costing: costing,
		Units:   "kilometers",
	})
	if err != nil {
		return DistanceResult{}, err
	}
	if len(response.SourcesToTargets) != 1 || len(response.SourcesToTargets[0]) != 1 {
		return DistanceResult{}, fmt.Errorf("Valhalla response has unexpected matrix size")
	}

	element := response.SourcesToTargets[0][0]
	if element.Distance == nil {
		return DistanceResult{}, fmt.Errorf("Valhalla response distance is null for the requested pair")
	}
	if element.Time == nil {
		return DistanceResult{}, fmt.Errorf("Valhalla response time is null for the requested pair")
	}

	return DistanceResult{
		DistanceKM:  *element.Distance,
		TimeSeconds: *element.Time,
	}, nil
}

func normalizeMatrixRequest(request MatrixRequest) MatrixRequest {
	if request.Costing == "" {
		request.Costing = "auto"
	}
	if request.Units == "" {
		request.Units = "kilometers"
	}
	return request
}

func validateMatrixRequest(request MatrixRequest) error {
	if len(request.Sources) == 0 {
		return fmt.Errorf("Valhalla matrix request must contain at least one source")
	}
	if len(request.Targets) == 0 {
		return fmt.Errorf("Valhalla matrix request must contain at least one target")
	}
	for index, location := range request.Sources {
		if err := validateLocation(location); err != nil {
			return fmt.Errorf("invalid source %d: %w", index, err)
		}
	}
	for index, location := range request.Targets {
		if err := validateLocation(location); err != nil {
			return fmt.Errorf("invalid target %d: %w", index, err)
		}
	}
	return nil
}

func validateLocation(location Location) error {
	if math.IsNaN(location.Lat) || location.Lat < -90 || location.Lat > 90 {
		return fmt.Errorf("latitude must be between -90 and 90")
	}
	if math.IsNaN(location.Lon) || location.Lon < -180 || location.Lon > 180 {
		return fmt.Errorf("longitude must be between -180 and 180")
	}
	return nil
}
