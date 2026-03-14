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

func renderContentPanel(content string, width int) string {
	if width > 80 {
		width = 80
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

	screen    screen
	selected  int
	projectID string
	ticketID  string
	err       string
	width     int
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
	return loadProjects(m.api, m.orgID)
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
		case "b", "esc":
			return m.handleBack()
		case "a":
			if m.screen == screenReviewDecision {
				return m, submitReview(m.api, m.ticketID, "approved")
			}
		case "r":
			if m.screen == screenReviewDecision {
				return m, submitReview(m.api, m.ticketID, "rejected")
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
		return m, loadOrgs(m.api)
	case orgsMsg:
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
		return m, loadProjects(m.api, m.orgID)
	case projectsMsg:
		m.projects = msg.projects
		m.err = msg.err
		m.screen = screenProjects
		if len(m.projects) > 0 {
			m.selected = 0
		}
		return m, nil
	case ticketsMsg:
		m.tickets = msg.tickets
		m.err = msg.err
		m.screen = screenTickets
		if len(m.tickets) > 0 {
			m.selected = 0
		}
		return m, nil
	case reviewsMsg:
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
			return m, loadPendingReviews(m.api, m.projectID)
		}
		return m, nil
	case traceMsg:
		m.reviewTrace = msg.trace
		if msg.err != "" {
			m.err = "trace: " + msg.err
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
		return 3
	case screenTickets:
		return len(m.tickets)
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
			return m, loadTickets(m.api, m.projectID)
		case 1:
			return m, loadPendingReviews(m.api, m.projectID)
		case 2:
			m.screen = screenProjects
			m.selected = 0
		}
		return m, nil
	case screenTickets:
		return m, nil
	case screenPendingReviews:
		if m.selected >= 0 && m.selected < len(m.reviews) {
			t := &m.reviews[m.selected]
			if id := t.Id; id != nil {
				m.ticketID = *id
				m.reviewTicket = t
				m.reviewTrace = nil
				m.screen = screenReviewDecision
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
	case screenTickets, screenPendingReviews:
		m.screen = screenProjectMenu
		m.selected = 0
		return m, nil
	case screenReviewDecision:
		m.screen = screenPendingReviews
		m.reviewTicket = nil
		m.reviewTrace = nil
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
		loginBody := "Not logged in.\n\n  ▸ Log in with GitHub (opens browser)\n\nPress Enter to open the browser and sign in. You'll be redirected back here."
		width := m.width
		if width <= 0 {
			width = 80
		}
		b.WriteString(components.CardRender("", loginBody, width))
	case screenOrgSelect:
		items := make([]string, 0, len(m.orgs))
		for _, o := range m.orgs {
			items = append(items, str(o.Name)+" ("+str(o.Slug)+")")
		}
		list := components.SelectList{Items: items, Selected: m.selected, EmptyMessage: "(none – sign in with GitHub to get a default org)"}
		b.WriteString(components.Primary.Render("Select organization") + "\n\n")
		b.WriteString(list.Render(m.width))
	case screenProjects:
		items := make([]string, 0, len(m.projects))
		for _, p := range m.projects {
			items = append(items, str(p.Name)+" ("+str(p.Slug)+")")
		}
		list := components.SelectList{Items: items, Selected: m.selected, EmptyMessage: "(none)"}
		b.WriteString(components.Primary.Render("Projects") + "\n\n")
		b.WriteString(list.Render(m.width))
	case screenProjectMenu:
		menus := []string{"List tickets", "Pending reviews", "Back to projects"}
		list := components.SelectList{Items: menus, Selected: m.selected, EmptyMessage: ""}
		b.WriteString(components.Primary.Render("Project: " + m.projectID) + "\n\n")
		b.WriteString(list.Render(m.width))
	case screenTickets:
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
		items := make([]string, 0, len(m.reviews))
		for _, t := range m.reviews {
			items = append(items, str(t.Id)+" "+str(t.Title))
		}
		emptyMsg := "No tickets awaiting review.\n\nTickets move here when an agent marks them \"awaiting_review\".\nCreate tickets, claim and complete work, then submit for review."
		list := components.SelectList{Items: items, Selected: m.selected, EmptyMessage: emptyMsg}
		b.WriteString(components.Primary.Render("Pending reviews") + "\n\n")
		b.WriteString(list.Render(m.width))
	case screenReviewDecision:
		if m.reviewTicket != nil {
			var reviewBody strings.Builder
			t := m.reviewTicket
			reviewBody.WriteString("  " + components.Primary.Render(str(t.Title)) + "\n")
			if t.Id != nil {
				reviewBody.WriteString("  ID: " + *t.Id + "\n")
			}
			if t.State != nil {
				reviewBody.WriteString("  State: " + string(*t.State) + "\n")
			}
			if t.Objective != nil && t.Objective.Description != nil && *t.Objective.Description != "" {
				reviewBody.WriteString("\n  ")
				reviewBody.WriteString(*t.Objective.Description)
				reviewBody.WriteString("\n")
			}
			reviewBody.WriteString("\n  " + components.Primary.Render("Outputs (from agent)") + "\n")
			if t.Outputs != nil && len(*t.Outputs) > 0 {
				reviewBody.WriteString(formatOutputs(*t.Outputs))
			} else {
				reviewBody.WriteString("  (none — agent should call submit_ticket with outputs)\n")
			}
			reviewBody.WriteString("\n  " + components.Primary.Render("Execution trace") + "\n")
			if m.reviewTrace != nil && m.reviewTrace.Steps != nil && len(*m.reviewTrace.Steps) > 0 {
				for _, s := range *m.reviewTrace.Steps {
					typ := "?"
					if s.Type != nil {
						typ = string(*s.Type)
					}
					reviewBody.WriteString("  - " + typ)
					if s.Payload != nil && len(*s.Payload) > 0 {
						reviewBody.WriteString(": " + formatPayloadShort(*s.Payload))
					}
					reviewBody.WriteString("\n")
				}
			} else {
				reviewBody.WriteString("  (no steps — agent should use log_step while working)\n")
			}
			rw := m.width
			if rw <= 0 {
				rw = 80
			}
			b.WriteString(components.CardRender("Review ticket", reviewBody.String(), rw))
		}
		b.WriteString("\n  [a] Approve  [r] Reject  [b] Back\n")
	}
	body := b.String()
	width := m.width
	if width <= 0 {
		width = 80
	}
	contentArea := renderContentPanel(body, width)
	helpHints := []string{"↑/k ↓/j select", "enter choose", "b/esc back", "q quit"}
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
	trace *client.ExecutionTrace
	err   string
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

func loadProjects(api *client.ClientWithResponses, orgID string) tea.Cmd {
	return func() tea.Msg {
		rsp, err := api.ListProjectsByOrgWithResponse(context.Background(), orgID, nil)
		if err != nil {
			return projectsMsg{err: err.Error()}
		}
		if rsp.JSON200 == nil {
			return projectsMsg{err: "no projects response"}
		}
		return projectsMsg{projects: *rsp.JSON200}
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
			return traceMsg{err: err.Error()}
		}
		if rsp.StatusCode() != 200 {
			return traceMsg{err: fmt.Sprintf("HTTP %d", rsp.StatusCode())}
		}
		if rsp.JSON200 == nil {
			return traceMsg{}
		}
		return traceMsg{trace: rsp.JSON200}
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
