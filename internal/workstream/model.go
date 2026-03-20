package workstream

import "time"

// WorkStream is a logical grouping of tickets toward a goal (e.g. "Productionize feature A").
// Status is "active" (default) or "closed". List endpoints default to active only.
type WorkStream struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Plan        string    `json:"plan,omitempty"`
	Branch      string    `json:"branch,omitempty"`
	Status      string    `json:"status"` // "active" or "closed"
	CreatedAt   time.Time `json:"created_at"`
}
