package opensky

import (
	"bytes"
	"strings"
	"time"
)

type errorResponse struct {
	Timestamp int64  `json:"timestamp"`
	Status    int    `json:"status"`
	Error     string `json:"error"`
	Exception string `json:"exception"`
	Message   string `json:"message"`
	Path      string `json:"path"`
}

// CallSignTrim is a string type that trims whitespace when unmarshalling from JSON.
type CallSignTrim string

// Flight represents a flight returned by the /flights/* endpoints.
type Flight struct {
	Icao24                           string       `json:"icao24"`
	FirstSeen                        int          `json:"firstSeen"`
	EstDepartureAirport              string       `json:"estDepartureAirport"`
	LastSeen                         int          `json:"lastSeen"`
	EstArrivalAirport                string       `json:"estArrivalAirport"`
	Callsign                         CallSignTrim `json:"callsign"`
	EstDepartureAirportHorizDistance int          `json:"estDepartureAirportHorizDistance"`
	EstDepartureAirportVertDistance  int          `json:"estDepartureAirportVertDistance"`
	EstArrivalAirportHorizDistance   int          `json:"estArrivalAirportHorizDistance"`
	EstArrivalAirportVertDistance    int          `json:"estArrivalAirportVertDistance"`
	DepartureAirportCandidatesCount  int          `json:"departureAirportCandidatesCount"`
	ArrivalAirportCandidatesCount    int          `json:"arrivalAirportCandidatesCount"`
}

// UnmarshalJSON trims quotes and whitespace from the raw JSON callsign value.
func (c *CallSignTrim) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*c = ""
		return nil
	}
	data = bytes.Trim(data, "\"")
	*c = CallSignTrim(strings.TrimSpace(string(data)))
	return nil
}

// unstructuredTrackResponse is the raw JSON shape returned by the tracks endpoint.
type unstructuredTrackResponse struct {
	Icao24    string  `json:"icao24"`
	Callsign  string  `json:"callsign"`
	StartTime float64 `json:"startTime"`
	EndTime   float64 `json:"endTime"`
	Paths     [][]any `json:"path"`
}

// GetTracksResponse is the parsed response returned by GetTrackByAircraft.
type GetTracksResponse struct {
	Icao24    string     `json:"icao24"`
	Callsign  string     `json:"callsign"`
	StartTime time.Time  `json:"startTime"`
	EndTime   time.Time  `json:"endTime"`
	Paths     []Waypoint `json:"path"`
}

// Waypoint represents a single point in an aircraft's trajectory.
type Waypoint struct {
	Time         time.Time `json:"time"`
	Latitude     *float64  `json:"latitude"`
	Longitude    *float64  `json:"longitude"`
	BaroAltitude *float64  `json:"baro_altitude"`
	TrueTrack    *float64  `json:"true_track"`
	OnGround     bool      `json:"on_ground"`
}

// PositionSource indicates the origin of a state vector's position.
type PositionSource int

const (
	PositionSourceADSB    PositionSource = 0
	PositionSourceASTERIX PositionSource = 1
	PositionSourceMLAT    PositionSource = 2
	PositionSourceFLARM   PositionSource = 3
)

// AircraftCategory describes the type of aircraft.
type AircraftCategory int

const (
	CategoryNoInfo           AircraftCategory = 0
	CategoryNoADSBInfo       AircraftCategory = 1
	CategoryLight            AircraftCategory = 2
	CategorySmall            AircraftCategory = 3
	CategoryLarge            AircraftCategory = 4
	CategoryHighVortexLarge  AircraftCategory = 5
	CategoryHeavy            AircraftCategory = 6
	CategoryHighPerformance  AircraftCategory = 7
	CategoryRotorcraft       AircraftCategory = 8
	CategoryGlider           AircraftCategory = 9
	CategoryLighterThanAir   AircraftCategory = 10
	CategoryParachutist      AircraftCategory = 11
	CategoryUltralight       AircraftCategory = 12
	CategoryReserved         AircraftCategory = 13
	CategoryUAV              AircraftCategory = 14
	CategorySpace            AircraftCategory = 15
	CategoryEmergencyVehicle AircraftCategory = 16
	CategoryServiceVehicle   AircraftCategory = 17
	CategoryPointObstacle    AircraftCategory = 18
	CategoryClusterObstacle  AircraftCategory = 19
	CategoryLineObstacle     AircraftCategory = 20
)

// StateVector represents the state of an aircraft at a given time.
type StateVector struct {
	Icao24         string           `json:"icao24"`
	Callsign       *string          `json:"callsign"`
	OriginCountry  string           `json:"origin_country"`
	TimePosition   *int64           `json:"time_position"`
	LastContact    int64            `json:"last_contact"`
	Longitude      *float64         `json:"longitude"`
	Latitude       *float64         `json:"latitude"`
	BaroAltitude   *float64         `json:"baro_altitude"`
	OnGround       bool             `json:"on_ground"`
	Velocity       *float64         `json:"velocity"`
	TrueTrack      *float64         `json:"true_track"`
	VerticalRate   *float64         `json:"vertical_rate"`
	Sensors        []int            `json:"sensors"`
	GeoAltitude    *float64         `json:"geo_altitude"`
	Squawk         *string          `json:"squawk"`
	Spi            bool             `json:"spi"`
	PositionSource PositionSource   `json:"position_source"`
	Category       AircraftCategory `json:"category"`
}

// StatesResponse is the response returned by the /states/all and /states/own endpoints.
type StatesResponse struct {
	Time   int64         `json:"time"`
	States []StateVector `json:"states"`
}

// BoundingBox defines a geographic area for filtering state vectors.
type BoundingBox struct {
	LaMin float64 // lower bound for latitude in decimal degrees
	LoMin float64 // lower bound for longitude in decimal degrees
	LaMax float64 // upper bound for latitude in decimal degrees
	LoMax float64 // upper bound for longitude in decimal degrees
}

// unstructuredStatesResponse is the raw JSON shape returned by the API.
// States are two-dimensional arrays that need manual parsing.
type unstructuredStatesResponse struct {
	Time   int64   `json:"time"`
	States [][]any `json:"states"`
}
