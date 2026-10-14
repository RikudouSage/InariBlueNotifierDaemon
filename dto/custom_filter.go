package dto

type CustomAppFilter struct {
	FilterName          string   `json:"name"`
	AndroidPackageNames []string `json:"androidPackageNames,omitempty"`
	NemoOwners          []string `json:"nemoOwners,omitempty"`
}
