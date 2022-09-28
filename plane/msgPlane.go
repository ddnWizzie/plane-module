package plane

import geo "github.com/kellydunn/golang-geo"

type Coordinate struct {
	X *float64 `json:"x,omitempty"`
	Y *float64 `json:"y,omitempty"`
}

type GeoCoordinate struct {
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

type GeoMarkers struct {
	Coordinate    Coordinate
	GeoCoordinate GeoCoordinate
}

type GeoCoordinates struct {
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

type Zones struct {
	Id              *string `json:"id,omitempty"`
	Identifier      *string `json:"identifier,omitempty"`
	Description     *string `json:"description,omitempty"`
	Floor           *string `json:"floor,omitempty"`
	Color           *string `json:"color,omitempty"`
	PlaneIdentifier *string `json:"planeIdentifier,omitempty"`
	PlaneId         *string `json:"planeId,omitempty"`
	GeoCoordinates  []GeoCoordinates
	Layouts         *string `json:"layouts,omitempty"`
	Planes          *string `json:"planes,omitempty"`
}

type Layouts struct {
	Id    *string `json:"id,omitempty"`
	Name  *string `json:"name,omitempty"`
	Zones []Zones `json:"zones,omitempty"`
}

type AllZones struct {
	AllZones []Zones `json:"allZones,omitempty"`
}

type InPlane struct {
	Id               *string `json:"id,omitempty"`
	Identifier       *string `json:"identifier,omitempty"`
	ImageContentType *string `json:"imageContentType,omitempty"`
	Image            *string `json:"image,omitempty"`
	Description      *string `json:"description,omitempty"`
	GeoMarkers       []GeoMarkers
	Layouts          []Layouts `json:"layouts,omitempty"`
	Zones            []Zones   `json:"zones,omitempty"`
	AllZones         []Zones   `json:"allZones,omitempty"`
}

type ZoneLayouts struct {
	LayoutsId     string
	LayoutsName   string
	ZoneIdLyots   string
	Poly          *geo.Polygon
	ZoneNameLyots string
}

type ZonePlane struct {
	ZoneIdZP   string
	Poly       *geo.Polygon
	ZoneNameZP string
}

type ZonePolygon struct {
	PlaneId     string
	PlaneName   string
	ZonePlane   []ZonePlane
	ZoneLayouts []ZoneLayouts
}
