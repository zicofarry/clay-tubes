package model

type GeoPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type MerchantDocument struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	Location     GeoPoint `json:"location"`
	Rating       float32  `json:"rating"`
	Tags         []string `json:"tags"`
	IsActive     bool     `json:"is_active"`
	CuisineTypes []string `json:"cuisine_types"`
}

type MenuItemDocument struct {
	ID           string   `json:"id"`
	MerchantID   string   `json:"merchant_id"`
	MerchantName string   `json:"merchant_name"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Price        float32  `json:"price"`
	Tags         []string `json:"tags"`
	CuisineType  string   `json:"cuisine_type"`
	IsAvailable  bool     `json:"is_available"`
}
