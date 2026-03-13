package rest

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/matt0x6f/warrant/internal/agent"
	apierrors "github.com/matt0x6f/warrant/internal/errors"
	"github.com/matt0x6f/warrant/internal/execution"
	"github.com/matt0x6f/warrant/internal/org"
	"github.com/matt0x6f/warrant/internal/project"
	"github.com/matt0x6f/warrant/internal/ticket"
)

// TraceHandler handles execution trace endpoints.
type TraceHandler struct {
	TraceSvc   *execution.Service
	TicketSvc  *ticket.Service
	ProjectSvc *project.Service
	OrgSvc     *org.Service
	AgentStore *agent.Store
}

func (h *TraceHandler) Register(r chi.Router) {
	r.Post("/tickets/{ticketID}/trace", h.logStep)
	r.Get("/tickets/{ticketID}/trace", h.getTrace)
}

func (h *TraceHandler) logStep(w http.ResponseWriter, r *http.Request) {
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
		LeaseToken string            `json:"lease_token"`
		Step       execution.Step   `json:"step"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteStructuredError(w, apierrors.New(apierrors.CodeInvalidInput, "invalid body", false))
		return
	}
	if body.LeaseToken == "" {
		WriteStructuredError(w, apierrors.New(apierrors.CodeInvalidInput, "lease_token required", false))
		return
	}
	if body.Step.Type == "" {
		WriteStructuredError(w, apierrors.New(apierrors.CodeInvalidInput, "step.type required", false))
		return
	}
	if body.Step.Payload == nil {
		body.Step.Payload = make(map[string]any)
	}
	if err := h.TraceSvc.LogStep(r.Context(), ticketID, body.LeaseToken, body.Step); err != nil {
		WriteStructuredError(w, TicketError(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TraceHandler) getTrace(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "ticketID")
	t, err := h.TicketSvc.GetTicket(r.Context(), ticketID)
	if err != nil {
		WriteStructuredError(w, apierrors.MapError(err))
		return
	}
	if !EnsureProjectAccess(r.Context(), w, t.ProjectID, h.AgentStore, h.OrgSvc, h.ProjectSvc) {
		return
	}
	trace, err := h.TraceSvc.GetTrace(r.Context(), ticketID)
	if err != nil {
		WriteStructuredError(w, apierrors.MapError(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(trace)
}
