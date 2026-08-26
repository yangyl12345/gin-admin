package schema

// Status is the minimal public status projection for the shop module.
type Status struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}
