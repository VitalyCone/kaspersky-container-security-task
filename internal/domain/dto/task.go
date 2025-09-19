package dto

type Task struct {
	ID          string `json:"id"`
	Payload     string `json:"payload"`
	MaxRetries int    `json:"max_retries"`
}
