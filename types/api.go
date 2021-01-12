package types

// APIResponse struct
type APIResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}
