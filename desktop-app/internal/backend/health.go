package backend

type HealthResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}
