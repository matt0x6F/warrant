package org

import "time"

// Role is a member's role in an organization.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// Org is an organization (tenant).
type Org struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}

// Member is an org member with a role.
type Member struct {
	OrgID  string
	UserID string
	Role   Role
}
