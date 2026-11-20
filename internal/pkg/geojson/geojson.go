// Package geojson holds the GeoJSON response types shared by the map endpoints.
package geojson

import "encoding/json"

// Feature keeps properties and geometry as the raw JSON produced by PostGIS.
type Feature struct {
	Type       string          `json:"type" example:"Feature"`
	Properties json.RawMessage `json:"properties" swaggertype:"object"`
	Geometry   json.RawMessage `json:"geometry" swaggertype:"object"`
}

type FeatureCollection struct {
	Type     string    `json:"type" example:"FeatureCollection"`
	Features []Feature `json:"features"`
}

// NewFeature writes a missing geometry as null.
func NewFeature(properties string, geometry *string) Feature {
	f := Feature{Type: "Feature", Properties: json.RawMessage(properties), Geometry: json.RawMessage("null")}
	if geometry != nil {
		f.Geometry = json.RawMessage(*geometry)
	}
	return f
}

// NewCollection writes an empty list as [], never null.
func NewCollection(features []Feature) FeatureCollection {
	if features == nil {
		features = []Feature{}
	}
	return FeatureCollection{Type: "FeatureCollection", Features: features}
}
