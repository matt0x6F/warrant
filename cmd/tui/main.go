// TUI client for Warrant. Uses the generated REST client.
// Optional: WARRANT_BASE_URL (default http://localhost:8080).
// On start: log in with GitHub (browser), then select an org. No JWT or org ID required in env.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/matt0x6f/warrant/api/client"
	"github.com/matt0x6f/warrant/cmd/tui/components"
)

const (
	baseURLDefault = "http://localhost:8080"
)

func renderHeader(m model) string {
	s := components.Primary.Render("Warrant")
	if m.orgID != "" {
		name := ""
		for _, o := range m.orgs {
			if o.Id != nil && *o.Id == m.orgID {
				if o.Name != nil {
					name = *o.Name
				} else if o.Slug != nil {
					name = *o.Slug
				}
				break
			}
		}
		if name == "" {
			name = m.orgID
		}
		s += components.Muted.Render(" · "+name)
	}
	if m.projectID != "" {
		slug := m.projectID
		for _, p := range m.projects {
			if p.Id != nil && *p.Id == m.projectID {
				if p.Slug != nil {
					slug = *p.Slug
				}
				break
			}
		}
		s += components.Muted.Render(" · "+slug)
	}
	return s + "\n"
}

// contentWidth returns width to use for content panel and cards. Uses full terminal width when set; default 120 when unknown; max 200.
func contentWidth(m model) int {
	w := m.width
	if w <= 0 {
		return 120
	}
	if w > 200 {
		return 200
	}
	return w
}

func renderContentPanel(content string, width int) string {
	if width <= 0 {
		width = 120
	}
	if width > 200 {
		width = 200
	}
	return components.Border.Width(width).Padding(1, 2).Render(content)
}

func renderHelpBar(hints []string) string {
	return components.KeyHintBar(hints) + "\n"
}

func main() {
	baseURL := os.Getenv("WARRANT_BASE_URL")
	if baseURL == "" {
		baseURL = baseURLDefault
	}
	p := tea.NewProgram(newModel(baseURL, ""), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "run: %v\n", err)
		os.Exit(1)
	}
}

type screen int

const (
	screenLogin screen = iota
	screenOrgSelect
	screenProjects
	screenProjectMenu
	screenTickets
	screenTicketDetail
	screenPendingReviews
	screenReviewDecision
)

type model struct {
	baseURL string
	token   string
	api     *client.ClientWithResponses

	orgID    string
	orgs     []client.Org
	projects []client.Project
	tickets     []client.Ticket
	reviews      []client.Ticket
	reviewTicket *client.Ticket        // ticket currently being reviewed
	reviewTrace  *client.ExecutionTrace // execution trace (loaded when entering review screen)
	detailTicket *client.Ticket        // ticket shown in detail view
	detailTrace  *client.ExecutionTrace // trace for detail view
	projectStatus string               // active, closed, all for project list filter

	screen    screen
	selected  int
	projectID string
	ticketID  string
	err          string
	loading      bool
	confirmReject bool
	width        int
	height    int
}

func newModel(baseURL, token string) model {
	m := model{baseURL: baseURL, token: token}
	if token != "" {
		m.api, _ = client.NewClientWithResponses(baseURL, m.authEditor())
	}
	return m
}

func (m *model) authEditor() client.ClientOption {
	token := m.token
	return client.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	})
}

func (m model) Init() tea.Cmd {
	if m.token == "" {
		return nil
	}
	if m.orgID == "" {
		return loadOrgs(m.api)
	}
	return loadProjects(m.api, m.orgID, m.projectStatus)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
			return m, nil
		case "down", "j":
			max := m.listLen()
			if m.selected < max-1 {
				m.selected++
			}
			return m, nil
		case "enter":
			return m.handleEnter()
		case "f":
			if m.screen == screenProjects {
				// Cycle project filter: active -> closed -> all -> active
				switch m.projectStatus {
				case "closed":
					m.projectStatus = "all"
				case "all":
					m.projectStatus = "active"
				case "active":
					m.projectStatus = "closed"
				default:
					m.projectStatus = "closed"
				}
				m.loading = true
				return m, loadProjects(m.api, m.orgID, m.projectStatus)
			}
		case "b", "esc":
			if m.screen == screenReviewDecision && m.confirmReject {
				m.confirmReject = false
				return m, nil
			}
			return m.handleBack()
		case "a":
			if m.screen == screenReviewDecision && !m.confirmReject {
				return m, submitReview(m.api, m.ticketID, "approved")
			}
		case "r":
			if m.screen == screenReviewDecision {
				if m.confirmReject {
					// already in confirm; r does nothing
				} else {
					m.confirmReject = true
					return m, nil
				}
			}
		case "y":
			if m.screen == screenReviewDecision && m.confirmReject {
				m.confirmReject = false
				return m, submitReview(m.api, m.ticketID, "rejected")
			}
		case "n":
			if m.screen == screenReviewDecision && m.confirmReject {
				m.confirmReject = false
				return m, nil
			}
		}
		return m, nil
	case tokenMsg:
		m.token = msg.token
		m.err = msg.err
		if msg.err != "" {
			return m, nil
		}
		var err error
		m.api, err = client.NewClientWithResponses(m.baseURL, m.authEditor())
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.screen = screenOrgSelect
		m.loading = true
		return m, loadOrgs(m.api)
	case orgsMsg:
		m.loading = false
		m.orgs = msg.orgs
		m.err = msg.err
		m.screen = screenOrgSelect
		if len(m.orgs) > 0 {
			m.selected = 0
		}
		return m, nil
	case orgSelectedMsg:
		m.orgID = msg.orgID
		m.screen = screenProjects
		m.selected = 0
		m.loading = true
		return m, loadProjects(m.api, m.orgID, m.projectStatus)
	case projectsMsg:
		m.loading = false
		m.projects = msg.projects
		m.err = msg.err
		m.screen = screenProjects
		if len(m.projects) > 0 {
			m.selected = 0
		}
		return m, nil
	case projectUpdatedMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == "" {
			m.loading = true
			return m, loadProjects(m.api, m.orgID, m.projectStatus)
		}
		return m, nil
	case ticketsMsg:
		m.loading = false
		m.tickets = msg.tickets
		m.err = msg.err
		m.screen = screenTickets
		if len(m.tickets) > 0 {
			m.selected = 0
		}
		return m, nil
	case reviewsMsg:
		m.loading = false
		m.reviews = msg.tickets
		m.err = msg.err
		m.screen = screenPendingReviews
		if len(m.reviews) > 0 {
			m.selected = 0
		}
		return m, nil
	case reviewDoneMsg:
		m.err = msg.err
		if msg.err == "" {
			m.reviewTicket = nil
			m.reviewTrace = nil
			m.screen = screenPendingReviews
			m.loading = true
			return m, loadPendingReviews(m.api, m.projectID)
		}
		return m, nil
	case traceMsg:
		m.loading = false
		if msg.err != "" {
			m.err = "trace: " + msg.err
			return m, nil
		}
		if m.screen == screenTicketDetail && m.detailTicket != nil && m.detailTicket.Id != nil && *m.detailTicket.Id == msg.ticketID {
			m.detailTrace = msg.trace
			return m, nil
		}
		if m.screen == screenReviewDecision && m.ticketID == msg.ticketID {
			m.reviewTrace = msg.trace
			return m, nil
		}
		return m, nil
	}
	return m, nil
}

func (m model) listLen() int {
	switch m.screen {
	case screenLogin:
		return 0
	case screenOrgSelect:
		return len(m.orgs)
	case screenProjects:
		return len(m.projects)
	case screenProjectMenu:
		return 4
	case screenTickets:
		return len(m.tickets)
	case screenTicketDetail:
		return 0
	case screenPendingReviews:
		return len(m.reviews)
	case screenReviewDecision:
		return 0
	}
	return 0
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenLogin:
		return m, startLoginFlow(m.baseURL)
	case screenOrgSelect:
		if m.selected >= 0 && m.selected < len(m.orgs) {
			if id := m.orgs[m.selected].Id; id != nil {
				return m, selectOrg(*id)
			}
		}
		return m, nil
	case screenProjects:
		if m.selected >= 0 && m.selected < len(m.projects) {
			if id := m.projects[m.selected].Id; id != nil {
				m.projectID = *id
			}
			m.screen = screenProjectMenu
			m.selected = 0
		}
		return m, nil
	case screenProjectMenu:
		switch m.selected {
		case 0:
			m.loading = true
			return m, loadTickets(m.api, m.projectID)
		case 1:
			m.loading = true
			return m, loadPendingReviews(m.api, m.projectID)
		case 2:
			// Close or Reopen project
			for _, p := range m.projects {
				if p.Id != nil && *p.Id == m.projectID {
					if p.Status != nil && *p.Status == client.ProjectStatusClosed {
						m.loading = true
						return m, updateProject(m.api, m.projectID, string(client.UpdateProjectRequestStatusActive))
					}
					m.loading = true
					return m, updateProject(m.api, m.projectID, string(client.UpdateProjectRequestStatusClosed))
				}
			}
			return m, nil
		case 3:
			m.screen = screenProjects
			m.selected = 0
		}
		return m, nil
	case screenTickets:
		if m.selected >= 0 && m.selected < len(m.tickets) {
			t := &m.tickets[m.selected]
			if id := t.Id; id != nil {
				m.detailTicket = t
				m.detailTrace = nil
				m.screen = screenTicketDetail
				m.loading = true
				return m, loadTrace(m.api, *id)
			}
		}
		return m, nil
	case screenPendingReviews:
		if m.selected >= 0 && m.selected < len(m.reviews) {
			t := &m.reviews[m.selected]
			if id := t.Id; id != nil {
				m.ticketID = *id
				m.reviewTicket = t
				m.reviewTrace = nil
				m.screen = screenReviewDecision
				m.loading = true
				return m, loadTrace(m.api, *id)
			}
		}
		return m, nil
	}
	return m, nil
}

func (m model) handleBack() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenOrgSelect:
		m.screen = screenLogin
		m.token = ""
		m.api = nil
		m.orgID = ""
		return m, nil
	case screenProjectMenu:
		m.screen = screenProjects
		m.selected = 0
		return m, nil
	case screenTicketDetail:
		m.screen = screenTickets
		m.detailTicket = nil
		m.detailTrace = nil
		return m, nil
	case screenTickets, screenPendingReviews:
		m.screen = screenProjectMenu
		m.selected = 0
		return m, nil
	case screenReviewDecision:
		m.screen = screenPendingReviews
		m.reviewTicket = nil
		m.reviewTrace = nil
		m.confirmReject = false
		return m, nil
	case screenProjects:
		m.screen = screenOrgSelect
		m.projectID = ""
		return m, nil
	}
	return m, tea.Quit
}

func (m model) View() string {
	var b strings.Builder
	if m.err != "" {
		b.WriteString(components.Error.Render(m.err) + "\n\n")
	}
	switch m.screen {
	case screenLogin:
		b.WriteString("Not logged in.\n\n")
		b.WriteString(components.Primary.Render("Log in with GitHub") + " (opens browser)\n\n")
		b.WriteString(components.Muted.Render("Press Enter to open the browser and sign in. You'll be redirected back here."))
	case screenOrgSelect:
		if m.loading {
			b.WriteString(components.Muted.Render("Loading…"))
			break
		}
		items := make([]string, 0, len(m.orgs))
		for _, o := range m.orgs {
			items = append(items, str(o.Name)+" ("+str(o.Slug)+")")
		}
		list := components.SelectList{Items: items, Selected: m.selected, EmptyMessage: "(none – sign in with GitHub to get a default org)"}
		b.WriteString(components.Primary.Render("Select organization") + "\n\n")
		b.WriteString(list.Render(m.width))
	case screenProjects:
		if m.loading {
			b.WriteString(components.Muted.Render("Loading…"))
			break
		}
		items := make([]string, 0, len(m.projects))
		for _, p := range m.projects {
			items = append(items, str(p.Name)+" ("+str(p.Slug)+")")
		}
		list := components.SelectList{Items: items, Selected: m.selected, EmptyMessage: "(none)"}
		b.WriteString(components.Primary.Render("Projects") + "\n\n")
		b.WriteString(list.Render(m.width))
	case screenProjectMenu:
		closeReopen := "Close project"
		for _, p := range m.projects {
			if p.Id != nil && *p.Id == m.projectID && p.Status != nil && *p.Status == client.ProjectStatusClosed {
				closeReopen = "Reopen project"
				break
			}
		}
		menus := []string{"List tickets", "Pending reviews", closeReopen, "Back to projects"}
		list := components.SelectList{Items: menus, Selected: m.selected, EmptyMessage: ""}
		b.WriteString(components.Primary.Render("Project: " + m.projectID) + "\n\n")
		b.WriteString(list.Render(m.width))
	case screenTickets:
		if m.loading {
			b.WriteString(components.Muted.Render("Loading…"))
			break
		}
		items := make([]string, 0, len(m.tickets))
		for _, t := range m.tickets {
			state := ""
			if t.State != nil {
				state = string(*t.State)
			}
			items = append(items, fmt.Sprintf("%s [%s] %s", str(t.Id), state, str(t.Title)))
		}
		list := components.SelectList{Items: items, Selected: m.selected, EmptyMessage: "(none)"}
		b.WriteString(components.Primary.Render("Tickets") + "\n\n")
		b.WriteString(list.Render(m.width))
	case screenPendingReviews:
		if m.loading {
			b.WriteString(components.Muted.Render("Loading…"))
			break
		}
		items := make([]string, 0, len(m.reviews))
		for _, t := range m.reviews {
			items = append(items, str(t.Id)+" "+str(t.Title))
		}
		emptyMsg := "No tickets awaiting review.\n\nTickets move here when an agent marks them \"awaiting_review\".\nCreate tickets, claim and complete work, then submit for review."
		list := components.SelectList{Items: items, Selected: m.selected, EmptyMessage: emptyMsg}
		b.WriteString(components.Primary.Render("Pending reviews") + "\n\n")
		b.WriteString(list.Render(m.width))
	case screenTicketDetail:
		if m.loading && m.detailTrace == nil {
			b.WriteString(components.Muted.Render("Loading trace…"))
			break
		}
		if m.detailTicket != nil {
			w := contentWidth(m)
			body := formatTicketBody(m.detailTicket, m.detailTrace, false, w)
			b.WriteString(components.CardRender("Ticket detail", body, w))
		}
		b.WriteString("\n  [b] Back\n")
	case screenReviewDecision:
		if m.confirmReject {
			b.WriteString(components.Muted.Render("Reject this ticket? [y/N]"))
			break
		}
		if m.loading {
			b.WriteString(components.Muted.Render("Loading…"))
			break
		}
		if m.reviewTicket != nil {
			w := contentWidth(m)
			body := formatTicketBody(m.reviewTicket, m.reviewTrace, true, w)
			b.WriteString(components.CardRender("Review ticket", body, w))
		}
		b.WriteString("\n  [a] Approve  [r] Reject  [b] Back\n")
	}
	body := b.String()
	w := contentWidth(m)
	var contentArea string
	if m.screen == screenTicketDetail || m.screen == screenReviewDecision {
		// Single card, no outer panel (avoids double border and uses full width)
		contentArea = body
	} else {
		contentArea = renderContentPanel(body, w)
	}
	helpHints := []string{"↑/k ↓/j select", "enter choose", "b/esc back", "q quit"}
	if m.screen == screenReviewDecision && m.confirmReject {
		helpHints = []string{"y confirm", "n/Esc cancel"}
	} else if m.screen == screenReviewDecision {
		helpHints = []string{"a approve", "r reject", "b back"}
	} else if m.screen == screenTicketDetail {
		helpHints = []string{"b back"}
	} else if m.screen == screenProjects {
		filterLabel := "active"
		if m.projectStatus != "" {
			filterLabel = m.projectStatus
		}
		helpHints = []string{"↑/k ↓/j select", "enter choose", "f filter (" + filterLabel + ")", "b/esc back", "q quit"}
	}
	return renderHeader(m) + contentArea + "\n" + renderHelpBar(helpHints)
}

type tokenMsg struct {
	token string
	err   string
}
type orgsMsg struct {
	orgs []client.Org
	err  string
}
type orgSelectedMsg struct {
	orgID string
}
type projectsMsg struct {
	projects []client.Project
	err      string
}
type ticketsMsg struct {
	tickets []client.Ticket
	err     string
}
type reviewsMsg struct {
	tickets []client.Ticket
	err     string
}
type reviewDoneMsg struct {
	err string
}
type traceMsg struct {
	ticketID string
	trace    *client.ExecutionTrace
	err      string
}
type projectUpdatedMsg struct {
	err string
}

func startLoginFlow(baseURL string) tea.Cmd {
	return func() tea.Msg {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return tokenMsg{err: "could not start callback server: " + err.Error()}
		}
		port := listener.Addr().(*net.TCPAddr).Port
		callbackURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)
		loginURL := baseURL + "/auth/github?redirect_uri=" + url.QueryEscape(callbackURL)

		// Pre-flight: hit login URL before opening browser. Server must return 302 to GitHub.
		client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := client.Get(loginURL)
		if err != nil {
			return tokenMsg{err: "server unreachable at " + baseURL + ": " + err.Error()}
		}
		if resp.StatusCode != http.StatusTemporaryRedirect && resp.StatusCode != http.StatusFound {
			body := ""
			if resp.Body != nil {
				buf := make([]byte, 1024)
				n, _ := resp.Body.Read(buf)
				if n > 0 {
					body = ": " + strings.TrimSpace(string(buf[:n]))
				}
				resp.Body.Close()
			}
			msg := "login failed: server returned " + resp.Status + body
			if body == "" {
				msg += ". Run the server from this repo with 'go run ./cmd/server' (it loads .env) and try again."
			} else {
				msg += ". Fix the server error and try again."
			}
			return tokenMsg{err: msg}
		}
		resp.Body.Close()

		ch := make(chan string, 1)
		srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/callback" {
				http.NotFound(w, r)
				return
			}
			token := r.URL.Query().Get("token")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>Warrant</title></head><body><p>Success! You can close this window and return to the TUI.</p></body></html>`))
			select {
			case ch <- token:
			default:
			}
		})}
		go func() {
			_ = srv.Serve(listener)
		}()
		openURL(loginURL)
		var token string
		select {
		case token = <-ch:
		case <-time.After(2 * time.Minute):
			token = ""
		}
		_ = srv.Close()
		if token == "" {
			return tokenMsg{err: "no token received (did you complete sign-in? If the browser showed an error, restart the server and try again.)"}
		}
		return tokenMsg{token: token}
	}
}

func openURL(url string) {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("open", url).Start()
	case "linux":
		_ = exec.Command("xdg-open", url).Start()
	case "windows":
		_ = exec.Command("cmd", "/c", "start", url).Start()
	}
}

func selectOrg(orgID string) tea.Cmd {
	return func() tea.Msg {
		return orgSelectedMsg{orgID: orgID}
	}
}

func loadOrgs(api *client.ClientWithResponses) tea.Cmd {
	return func() tea.Msg {
		rsp, err := api.ListOrgsWithResponse(context.Background())
		if err != nil {
			return orgsMsg{err: err.Error()}
		}
		if rsp.JSON200 == nil {
			return orgsMsg{err: "not authorized (list orgs requires OAuth)"}
		}
		return orgsMsg{orgs: *rsp.JSON200}
	}
}

func loadProjects(api *client.ClientWithResponses, orgID, status string) tea.Cmd {
	return func() tea.Msg {
		var params *client.ListProjectsByOrgParams
		if status != "" {
			s := client.ListProjectsByOrgParamsStatus(status)
			params = &client.ListProjectsByOrgParams{Status: &s}
		}
		rsp, err := api.ListProjectsByOrgWithResponse(context.Background(), orgID, params)
		if err != nil {
			return projectsMsg{err: err.Error()}
		}
		if rsp.JSON200 == nil {
			return projectsMsg{err: "no projects response"}
		}
		return projectsMsg{projects: *rsp.JSON200}
	}
}

func updateProject(api *client.ClientWithResponses, projectID, status string) tea.Cmd {
	return func() tea.Msg {
		s := client.UpdateProjectRequestStatus(status)
		body := client.UpdateProjectJSONRequestBody{Status: &s}
		rsp, err := api.UpdateProjectWithResponse(context.Background(), projectID, body)
		if err != nil {
			return projectUpdatedMsg{err: err.Error()}
		}
		if rsp.StatusCode() >= 400 {
			return projectUpdatedMsg{err: fmt.Sprintf("HTTP %d", rsp.StatusCode())}
		}
		return projectUpdatedMsg{}
	}
}

func loadTickets(api *client.ClientWithResponses, projectID string) tea.Cmd {
	return func() tea.Msg {
		rsp, err := api.ListTicketsWithResponse(context.Background(), projectID)
		if err != nil {
			return ticketsMsg{err: err.Error()}
		}
		if rsp.StatusCode() != 200 {
			return ticketsMsg{err: fmt.Sprintf("HTTP %d", rsp.StatusCode())}
		}
		var tickets []client.Ticket
		if err := json.Unmarshal(rsp.Body, &tickets); err != nil {
			return ticketsMsg{err: err.Error()}
		}
		return ticketsMsg{tickets: tickets}
	}
}

func loadPendingReviews(api *client.ClientWithResponses, projectID string) tea.Cmd {
	return func() tea.Msg {
		rsp, err := api.ListPendingReviewsWithResponse(context.Background(), projectID)
		if err != nil {
			return reviewsMsg{err: err.Error()}
		}
		if rsp.JSON200 == nil {
			return reviewsMsg{err: "no reviews response"}
		}
		tickets := rsp.JSON200.Tickets
		if tickets == nil {
			return reviewsMsg{tickets: []client.Ticket{}}
		}
		return reviewsMsg{tickets: *tickets}
	}
}

func loadTrace(api *client.ClientWithResponses, ticketID string) tea.Cmd {
	return func() tea.Msg {
		rsp, err := api.GetTraceWithResponse(context.Background(), ticketID)
		if err != nil {
			return traceMsg{ticketID: ticketID, err: err.Error()}
		}
		if rsp.StatusCode() != 200 {
			return traceMsg{ticketID: ticketID, err: fmt.Sprintf("HTTP %d", rsp.StatusCode())}
		}
		if rsp.JSON200 == nil {
			return traceMsg{ticketID: ticketID}
		}
		return traceMsg{ticketID: ticketID, trace: rsp.JSON200}
	}
}

func submitReview(api *client.ClientWithResponses, ticketID, decision string) tea.Cmd {
	return func() tea.Msg {
		body := client.CreateReviewJSONRequestBody{
			Decision:   client.CreateReviewRequestDecision(decision),
			ReviewerId: ptr("tui"),
		}
		rsp, err := api.CreateReviewWithResponse(context.Background(), ticketID, body)
		if err != nil {
			return reviewDoneMsg{err: err.Error()}
		}
		if rsp.StatusCode() >= 400 {
			return reviewDoneMsg{err: fmt.Sprintf("HTTP %d", rsp.StatusCode())}
		}
		return reviewDoneMsg{}
	}
}

func ptr(s string) *string { return &s }

func str(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func formatOutputs(m map[string]interface{}) string {
	if len(m) == 0 {
		return ""
	}
	var lines []string
	if v, ok := m["summary"].(string); ok && v != "" {
		lines = append(lines, "  "+v)
	}
	for k, v := range m {
		if k == "summary" {
			continue
		}
		lines = append(lines, "  "+k+": "+fmt.Sprint(v))
	}
	if len(lines) > 8 {
		lines = lines[:8]
		lines = append(lines, "  ...")
	}
	return strings.Join(lines, "\n") + "\n"
}

// formatOutputsFull is like formatOutputs but shows all keys (no 8-line cap) for rich view.
func formatOutputsFull(m map[string]interface{}) string {
	if len(m) == 0 {
		return ""
	}
	var lines []string
	if v, ok := m["summary"].(string); ok && v != "" {
		lines = append(lines, "  "+v)
	}
	for k, v := range m {
		if k == "summary" {
			continue
		}
		lines = append(lines, "  "+k+": "+fmt.Sprint(v))
	}
	return strings.Join(lines, "\n") + "\n"
}

// maxPayloadLen is the max characters shown for a trace step payload (summary/name/JSON).
const maxPayloadLen = 200

func formatPayloadShort(m map[string]interface{}) string {
	if v, ok := m["name"].(string); ok && v != "" {
		if len(v) > maxPayloadLen {
			return v[:maxPayloadLen-3] + "..."
		}
		return v
	}
	if v, ok := m["summary"].(string); ok && v != "" {
		if len(v) > maxPayloadLen {
			return v[:maxPayloadLen-3] + "..."
		}
		return v
	}
	b, _ := json.Marshal(m)
	s := string(b)
	if len(s) > maxPayloadLen {
		return s[:maxPayloadLen-3] + "..."
	}
	return s
}

// formatTicketBody builds rich ticket content for detail and review views (objective, context, outputs, trace).
// width is the content area width; wrapping uses width-6 for card inner width.
func formatTicketBody(t *client.Ticket, trace *client.ExecutionTrace, forReview bool, width int) string {
	if t == nil {
		return ""
	}
	innerWidth := width - 6
	if innerWidth < 40 {
		innerWidth = 40
	}
	var b strings.Builder
	// Meta
	b.WriteString("  " + components.Primary.Render(str(t.Title)) + "\n")
	if t.Id != nil {
		b.WriteString("  ID: " + *t.Id + "\n")
	}
	var meta []string
	if t.State != nil {
		meta = append(meta, "State: "+string(*t.State))
	}
	if t.Type != nil {
		meta = append(meta, "Type: "+string(*t.Type))
	}
	if t.Priority != nil {
		meta = append(meta, "Priority: "+fmt.Sprint(*t.Priority))
	}
	if len(meta) > 0 {
		b.WriteString("  " + strings.Join(meta, "  ") + "\n")
	}
	if t.AssignedTo != nil && *t.AssignedTo != "" {
		b.WriteString("  Assigned to: " + *t.AssignedTo + "\n")
	}
	if t.CreatedBy != nil && *t.CreatedBy != "" {
		b.WriteString("  Created by: " + *t.CreatedBy + "\n")
	}
	if t.CreatedAt != nil {
		b.WriteString("  Created: " + t.CreatedAt.Format(time.RFC3339) + "\n")
	}
	if t.UpdatedAt != nil {
		b.WriteString("  Updated: " + t.UpdatedAt.Format(time.RFC3339) + "\n")
	}
	if t.DependsOn != nil && len(*t.DependsOn) > 0 {
		b.WriteString("  Depends on: " + strings.Join(*t.DependsOn, ", ") + "\n")
	}
	// Objective
	if t.Objective != nil {
		obj := t.Objective
		if obj.Description != nil && *obj.Description != "" {
			b.WriteString("\n  " + components.Primary.Render("Objective") + "\n")
			b.WriteString(wrapParagraph(*obj.Description, innerWidth, "  "))
			b.WriteString("\n")
		}
		if obj.SuccessCriteria != nil && len(*obj.SuccessCriteria) > 0 {
			b.WriteString("  Success criteria:\n")
			for _, c := range *obj.SuccessCriteria {
				b.WriteString("    • " + c + "\n")
			}
		}
		if obj.AcceptanceTest != nil && *obj.AcceptanceTest != "" {
			label := "  Acceptance test: "
			wrapped := wrapParagraph(*obj.AcceptanceTest, innerWidth-len(label), "  ")
			b.WriteString(label)
			b.WriteString(strings.TrimPrefix(wrapped, "  "))
			b.WriteString("\n")
		}
	}
	// Ticket context
	if t.TicketContext != nil {
		ctx := t.TicketContext
		hasCtx := false
		if ctx.RelevantFiles != nil && len(*ctx.RelevantFiles) > 0 {
			b.WriteString("\n  " + components.Primary.Render("Relevant files") + "\n")
			for _, f := range *ctx.RelevantFiles {
				b.WriteString("    " + f + "\n")
			}
			hasCtx = true
		}
		if ctx.Constraints != nil && len(*ctx.Constraints) > 0 {
			b.WriteString("\n  " + components.Primary.Render("Constraints") + "\n")
			for _, c := range *ctx.Constraints {
				b.WriteString("    • " + c + "\n")
			}
			hasCtx = true
		}
		if ctx.HumanAnswers != nil && len(*ctx.HumanAnswers) > 0 {
			b.WriteString("\n  " + components.Primary.Render("Human answers") + "\n")
			for i, a := range *ctx.HumanAnswers {
				b.WriteString("    " + fmt.Sprint(i+1) + ". " + a + "\n")
			}
			hasCtx = true
		}
		if ctx.PriorAttempts != nil && len(*ctx.PriorAttempts) > 0 {
			b.WriteString("\n  " + components.Primary.Render("Prior attempts") + "\n")
			for i, pa := range *ctx.PriorAttempts {
				b.WriteString("    Attempt " + fmt.Sprint(i+1) + ": ")
				b.WriteString(formatPayloadShort(pa) + "\n")
			}
			hasCtx = true
		}
		if hasCtx {
			b.WriteString("\n")
		}
	}
	// Inputs
	if t.Inputs != nil && len(*t.Inputs) > 0 {
		b.WriteString("\n  " + components.Primary.Render("Inputs") + "\n")
		b.WriteString(formatOutputs(*t.Inputs))
	}
	// Outputs (from agent)
	b.WriteString("\n  " + components.Primary.Render("Outputs (from agent)") + "\n")
	if t.Outputs != nil && len(*t.Outputs) > 0 {
		raw := formatOutputsFull(*t.Outputs)
		b.WriteString(wrapOutputsLines(raw, innerWidth))
		b.WriteString("\n")
	} else {
		if forReview {
			b.WriteString("  (none — agent should call submit_ticket with outputs)\n")
		} else {
			b.WriteString("  (none)\n")
		}
	}
	// Execution trace
	b.WriteString("\n  " + components.Primary.Render("Execution trace") + "\n")
	if trace != nil && trace.AgentId != nil && *trace.AgentId != "" {
		b.WriteString("  Agent: " + *trace.AgentId + "\n")
	}
	if trace != nil && trace.Steps != nil && len(*trace.Steps) > 0 {
		for _, s := range *trace.Steps {
			typ := "?"
			if s.Type != nil {
				typ = string(*s.Type)
			}
			b.WriteString("  - " + typ)
			if s.CreatedAt != nil {
				b.WriteString(" " + components.Muted.Render(s.CreatedAt.Format("15:04:05")))
			}
			if s.Payload != nil && len(*s.Payload) > 0 {
				b.WriteString("\n    " + formatPayloadShort(*s.Payload))
			}
			b.WriteString("\n")
		}
	} else {
		if forReview {
			b.WriteString("  (no steps — agent should use log_step while working)\n")
		} else {
			b.WriteString("  (no steps)\n")
		}
	}
	return b.String()
}

// wrapParagraph wraps s at word boundaries so each line is at most lineWidth runes; every line (including continuations) is prefixed with indent for consistent alignment.
func wrapParagraph(s string, lineWidth int, indent string) string {
	if lineWidth <= len(indent) {
		return s
	}
	contentWidth := lineWidth - len(indent)
	words := strings.Fields(s)
	if len(words) == 0 {
		return indent
	}
	var out strings.Builder
	out.WriteString(indent)
	lineLen := 0
	for _, w := range words {
		if lineLen > 0 && lineLen+1+len(w) > contentWidth {
			out.WriteString("\n" + indent)
			lineLen = 0
		}
		if lineLen > 0 {
			out.WriteString(" ")
			lineLen++
		}
		out.WriteString(w)
		lineLen += len(w)
	}
	return out.String()
}

// wrapOutputsLines wraps each line of s; lines are assumed to start with "  ", which is stripped before wrapping so continuations get the same "  " indent.
func wrapOutputsLines(s string, lineWidth int) string {
	lines := strings.Split(s, "\n")
	var out strings.Builder
	for i, line := range lines {
		if i > 0 {
			out.WriteString("\n")
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}
		rest := strings.TrimPrefix(line, "  ")
		if len(rest) < len(line) {
			out.WriteString(wrapParagraph(rest, lineWidth, "  "))
		} else {
			out.WriteString(wrapParagraph(line, lineWidth, "  "))
		}
	}
	return out.String()
}
