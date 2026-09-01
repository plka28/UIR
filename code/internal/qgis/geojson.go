package qgis

import (
	"fmt"
	"math"
)

const (
	geoJSONFeatureCollection = "FeatureCollection"
	geoJSONFeature           = "Feature"
	geoJSONPoint             = "Point"
	geoJSONLineString        = "LineString"
)

// FeatureCollection is the root GeoJSON object written for each QGIS layer.
type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

// Feature is one GeoJSON feature with point geometry and tabular attributes.
type Feature struct {
	Type       string         `json:"type"`
	Geometry   any            `json:"geometry"`
	Properties map[string]any `json:"properties"`
}

// PointGeometry stores coordinates in GeoJSON order: longitude, latitude.
type PointGeometry struct {
	Type        string     `json:"type"`
	Coordinates [2]float64 `json:"coordinates"`
}

// LineStringGeometry stores route endpoints as GeoJSON longitude-latitude pairs.
type LineStringGeometry struct {
	Type        string       `json:"type"`
	Coordinates [][2]float64 `json:"coordinates"`
}

// PointFeature is a normalized point record ready to be converted to GeoJSON.
type PointFeature struct {
	Latitude   float64
	Longitude  float64
	Properties map[string]any
}

// LineFeature is a normalized line record ready to be converted to GeoJSON.
type LineFeature struct {
	Coordinates [][2]float64
	Properties  map[string]any
}

// NewPointFeatureCollection converts prepared point records to GeoJSON.
func NewPointFeatureCollection(points []PointFeature) (FeatureCollection, error) {
	collection := FeatureCollection{
		Type:     geoJSONFeatureCollection,
		Features: make([]Feature, 0, len(points)),
	}

	for index, point := range points {
		if err := validateCoordinates(point.Latitude, point.Longitude); err != nil {
			return FeatureCollection{}, fmt.Errorf("point %d: %w", index+1, err)
		}

		collection.Features = append(collection.Features, Feature{
			Type: geoJSONFeature,
			Geometry: PointGeometry{
				Type:        geoJSONPoint,
				Coordinates: [2]float64{point.Longitude, point.Latitude},
			},
			Properties: copyProperties(point.Properties),
		})
	}

	return collection, nil
}

// NewLineFeatureCollection converts assignment or flow records to GeoJSON lines.
func NewLineFeatureCollection(lines []LineFeature) (FeatureCollection, error) {
	collection := FeatureCollection{
		Type:     geoJSONFeatureCollection,
		Features: make([]Feature, 0, len(lines)),
	}

	for index, line := range lines {
		if len(line.Coordinates) < 2 {
			return FeatureCollection{}, fmt.Errorf("line %d: line must contain at least two coordinates", index+1)
		}
		for coordinateIndex, coordinate := range line.Coordinates {
			if err := validateCoordinates(coordinate[1], coordinate[0]); err != nil {
				return FeatureCollection{}, fmt.Errorf("line %d coordinate %d: %w", index+1, coordinateIndex+1, err)
			}
		}

		collection.Features = append(collection.Features, Feature{
			Type: geoJSONFeature,
			Geometry: LineStringGeometry{
				Type:        geoJSONLineString,
				Coordinates: line.Coordinates,
			},
			Properties: copyProperties(line.Properties),
		})
	}

	return collection, nil
}

func validateCoordinates(latitude float64, longitude float64) error {
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) || latitude < -90 || latitude > 90 {
		return fmt.Errorf("latitude must be a finite value between -90 and 90")
	}
	if math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < -180 || longitude > 180 {
		return fmt.Errorf("longitude must be a finite value between -180 and 180")
	}
	return nil
}

func copyProperties(properties map[string]any) map[string]any {
	copied := make(map[string]any, len(properties))
	for key, value := range properties {
		copied[key] = value
	}
	return copied
}
