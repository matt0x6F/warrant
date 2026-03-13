package rest

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/matt0x6f/warrant/internal/agent"
	apierrors "github.com/matt0x6f/warrant/internal/errors"
	"github.com/matt0x6f/warrant/internal/org"
	"github.com/matt0x6f/warrant/internal/project"
	"github.com/matt0x6f/warrant/internal/review"
	"github.com/matt0x6f/warrant/internal/ticket"
)

// ReviewsHandler handles review and escalation REST endpoints.
type ReviewsHandler struct {
	ReviewSvc  *review.Service
	TicketSvc  interface {
		GetTicket(ctx context.Context, id string) (*ticket.Ticket, error)
	}
	ProjectSvc *project.Service
	OrgSvc     *org.Service
	AgentStore *agent.Store
}

func (h *ReviewsHandler) Register(r chi.Router) {
	r.Get("/projects/{projectID}/reviews", h.listPendingReviews)
	r.Post("/tickets/{ticketID}/reviews", h.createReview)
	r.Get("/projects/{projectID}/escalations", h.listEscalations)
	r.Post("/tickets/{ticketID}/escalations/{escalationID}/resolve", h.resolveEscalation)
}

func (h *ReviewsHandler) listPendingReviews(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !EnsureProjectAccess(r.Context(), w, projectID, h.AgentStore, h.OrgSvc, h.ProjectSvc) {
		return
	}
	ids, err := h.ReviewSvc.ListPendingReviews(r.Context(), projectID)
	if err != nil {
		WriteStructuredError(w, apierrors.MapError(err))
		return
	}
	tickets := make([]*ticket.Ticket, 0, len(ids))
	for _, id := range ids {
		t, _ := h.TicketSvc.GetTicket(r.Context(), id)
		if t != nil {
			tickets = append(tickets, t)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"tickets": tickets})
}

func (h *ReviewsHandler) createReview(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "ticketID")
	t, err := h.TicketSvc.GetTicket(r.Context(), ticketID)
	if err != nil {
		WriteStructuredError(w, apierrors.MapError(err))
		return
	}
	if !EnsureProjectAccess(r.Context(), w, t.ProjectID, h.AgentStore, h.OrgSvc, h.ProjectSvc) {
		return
	}
	var body struct {
		Decision   review.Decision `json:"decision"`
		Notes      string          `json:"notes"`
		ReviewerID string          `json:"reviewer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteStructuredError(w, apierrors.New(apierrors.CodeInvalidInput, "invalid body", false))
		return
	}
	if body.ReviewerID == "" {
		body.ReviewerID = "api"
	}
	switch body.Decision {
	case review.DecisionApproved:
		if err := h.ReviewSvc.ApproveTicket(r.Context(), ticketID, body.ReviewerID, body.Notes); err != nil {
			WriteStructuredError(w, TicketError(err))
			return
		}
	case review.DecisionRejected:
		if err := h.ReviewSvc.RejectTicket(r.Context(), ticketID, body.ReviewerID, body.Notes); err != nil {
			WriteStructuredError(w, TicketError(err))
			return
		}
	default:
		WriteStructuredError(w, apierrors.New(apierrors.CodeInvalidInput, "decision must be approved or rejected", false))
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ReviewsHandler) listEscalations(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !EnsureProjectAccess(r.Context(), w, projectID, h.AgentStore, h.OrgSvc, h.ProjectSvc) {
		return
	}
	list, err := h.ReviewSvc.ListEscalations(r.Context(), projectID)
	if err != nil {
		WriteStructuredError(w, apierrors.MapError(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (h *ReviewsHandler) resolveEscalation(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "ticketID")
	t, err := h.TicketSvc.GetTicket(r.Context(), ticketID)
	if err != nil {
		WriteStructuredError(w, apierrors.MapError(err))
		return
	}
	if !EnsureProjectAccess(r.Context(), w, t.ProjectID, h.AgentStore, h.OrgSvc, h.ProjectSvc) {
		return
	}
	escalationID := chi.URLParam(r, "escalationID")
	var body struct {
		Answer     string `json:"answer"`
		ReviewerID string `json:"reviewer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteStructuredError(w, apierrors.New(apierrors.CodeInvalidInput, "invalid body", false))
		return
	}
	if body.ReviewerID == "" {
		body.ReviewerID = "api"
	}
	if err := h.ReviewSvc.ResolveEscalation(r.Context(), ticketID, escalationID, body.ReviewerID, body.Answer); err != nil {
		WriteStructuredError(w, TicketError(err))
		return
	}
	w.WriteHeader(http.StatusOK)
}
