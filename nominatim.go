package main

type nominatimResponse struct {
	PlaceID     int              `json:"place_id"`
	Licence     string           `json:"licence"`
	OSMType     string           `json:"osm_type"`
	OSMID       int              `json:"osm_id"`
	Lat         string           `json:"lat"`
	Lon         string           `json:"lon"`
	PlaceRank   int              `json:"place_rank"`
	Category    string           `json:"category"`
	Type        string           `json:"type"`
	Importance  float64          `json:"importance"`
	AddressType string           `json:"addresstype"`
	Name        string           `json:"name"`
	DisplayName string           `json:"display_name"`
	Address     nominatimAddress `json:"address"`
	BoundingBox []string         `json:"boundingbox"`
}

type nominatimAddress struct {
	Path        string `json:"path"`
	Suburb      string `json:"suburb"`
	Village     string `json:"village"`
	City        string `json:"city"`
	County      string `json:"county"`
	State       string `json:"state"`
	PostCode    string `json:"postcode"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
}
