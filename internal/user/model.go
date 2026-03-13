package user

import "time"

// User is a GitHub identity, provisioned on first OAuth login.
type User struct {
	ID        string    `json:"id"`
	GitHubID  int64     `json:"github_id"`
	Login     string    `json:"login"`
	Name      string    `json:"name,omitempty"`
	Email     string    `json:"email,omitempty"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
