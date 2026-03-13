package rest

import (
	"context"
	"net/http"

	"github.com/matt0x6f/warrant/internal/agent"
	apierrors "github.com/matt0x6f/warrant/internal/errors"
	"github.com/matt0x6f/warrant/internal/project"
)

// AgentGetter is used by EnsureOrgAccess. *agent.Store implements it.
type AgentGetter interface {
	GetByID(ctx context.Context, id string) (*agent.Agent, error)
}

// OrgMemberLister is used by EnsureOrgAccess. *org.Service implements it.
type OrgMemberLister interface {
	ListOrgIDsForUser(ctx context.Context, userID string) ([]string, error)
}

// ProjectGetterForAccess is used by EnsureProjectAccess. *project.Service implements it.
type ProjectGetterForAccess interface {
	GetProject(ctx context.Context, projectID string) (*project.Project, error)
}

// EnsureOrgAccess requires an authenticated agent with OAuth (user link), verifies the user is a member of the given org,
// and writes 401/403 and returns false if not. Returns true when the caller has access.
func EnsureOrgAccess(ctx context.Context, w http.ResponseWriter, orgID string, agentStore AgentGetter, orgSvc OrgMemberLister) bool {
	agentID := GetAgentID(ctx)
	if agentID == "" {
		WriteStructuredError(w, apierrors.New(apierrors.CodeUnauthorized, "authentication required", false))
		return false
	}
	a, err := agentStore.GetByID(ctx, agentID)
	if err != nil || a == nil {
		WriteStructuredError(w, apierrors.New(apierrors.CodeUnauthorized, "agent not found", false))
		return false
	}
	if a.UserID == "" {
		WriteStructuredError(w, apierrors.New(apierrors.CodeUnauthorized, "OAuth required (agent must be linked to a user)", false))
		return false
	}
	orgIDs, err := orgSvc.ListOrgIDsForUser(ctx, a.UserID)
	if err != nil {
		WriteStructuredError(w, apierrors.MapError(err))
		return false
	}
	for _, id := range orgIDs {
		if id == orgID {
			return true
		}
	}
	WriteStructuredError(w, apierrors.New(apierrors.CodeForbidden, "you do not have access to that organization", false))
	return false
}

// EnsureProjectAccess requires an authenticated agent with OAuth, loads the project to get org_id, and verifies the user
// is a member of that org. Writes 401/403/404 and returns false if not. Returns true when the caller has access.
func EnsureProjectAccess(ctx context.Context, w http.ResponseWriter, projectID string, agentStore AgentGetter, orgSvc OrgMemberLister, projectSvc ProjectGetterForAccess) bool {
	proj, err := projectSvc.GetProject(ctx, projectID)
	if err != nil {
		WriteStructuredError(w, apierrors.MapError(err))
		return false
	}
	if proj == nil {
		WriteStructuredError(w, apierrors.New(apierrors.CodeNotFound, "project not found", false))
		return false
	}
	return EnsureOrgAccess(ctx, w, proj.OrgID, agentStore, orgSvc)
}
