package gitnotes

import (
	"os/exec"
	"strings"
	"testing"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH, skipping notes tests")
	}
}

// makeTempGitRepo creates a temp dir, runs git init and one commit, returns repo path and HEAD sha.
func makeTempGitRepo(t *testing.T) (repoPath, headSHA string) {
	t.Helper()
	dir := t.TempDir()
	for _, c := range []struct {
		args []string
		dir  string
	}{
		{[]string{"init"}, dir},
		{[]string{"config", "user.email", "test@test"}, dir},
		{[]string{"config", "user.name", "Test"}, dir},
		{[]string{"commit", "--allow-empty", "-m", "first"}, dir},
	} {
		cmd := exec.Command("git", c.args...)
		cmd.Dir = c.dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", c.args, err, out)
		}
	}
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	headSHA = strings.TrimSpace(string(out))
	return dir, headSHA
}

func TestAddNote_Validation(t *testing.T) {
	requireGit(t)
	ref := RefForType(TypeDecision)
	if ref == "" {
		t.Fatal("RefForType(decision) empty")
	}

	for _, tt := range []struct {
		name      string
		repoPath  string
		ref       string
		commitSHA string
		body      string
	}{
		{"missing repo", "", ref, "HEAD", "x"},
		{"missing ref", "/tmp", "", "HEAD", "x"},
		{"missing commit", "/tmp", ref, "", "x"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := AddNote(tt.repoPath, tt.ref, tt.commitSHA, tt.body)
			if err == nil {
				t.Error("expected error")
			}
			if !strings.Contains(err.Error(), "required") {
				t.Errorf("error should mention required: %v", err)
			}
		})
	}

	// Not a git repo
	notRepo := t.TempDir()
	err := AddNote(notRepo, ref, "HEAD", "body")
	if err == nil {
		t.Error("expected error for non-git dir")
	}
	if !strings.Contains(err.Error(), "not a git repo") {
		t.Errorf("error should mention not a git repo: %v", err)
	}
}

func TestAddNote_ShowNote_RoundTrip(t *testing.T) {
	requireGit(t)
	dir, headSHA := makeTempGitRepo(t)
	ref := RefForType(TypeDecision)
	body := `{"v":1,"type":"decision","message":"test"}`

	if err := AddNote(dir, ref, headSHA, body); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	got, err := ShowNote(dir, ref, headSHA)
	if err != nil {
		t.Fatalf("ShowNote: %v", err)
	}
	if got != body {
		t.Errorf("ShowNote = %q, want %q", got, body)
	}
}

func TestShowNote_Validation(t *testing.T) {
	requireGit(t)
	ref := RefForType(TypeTrace)
	_, err := ShowNote("", ref, "HEAD")
	if err == nil {
		t.Error("expected error for empty repoPath")
	}
	_, err = ShowNote("/tmp", "", "HEAD")
	if err == nil {
		t.Error("expected error for empty ref")
	}
	_, err = ShowNote("/tmp", ref, "")
	if err == nil {
		t.Error("expected error for empty commitSHA")
	}
}

func TestShowNote_NoNote(t *testing.T) {
	requireGit(t)
	dir, headSHA := makeTempGitRepo(t)
	ref := RefForType(TypeIntent)
	got, err := ShowNote(dir, ref, headSHA)
	if err != nil {
		t.Fatalf("ShowNote (no note): %v", err)
	}
	if got != "" {
		t.Errorf("ShowNote (no note) = %q, want empty", got)
	}
}

func TestLog_Validation(t *testing.T) {
	requireGit(t)
	ref := RefForType(TypeDecision)
	_, err := Log("", ref, 10)
	if err == nil {
		t.Error("expected error for empty repoPath")
	}
	_, err = Log("/tmp", "", 10)
	if err == nil {
		t.Error("expected error for empty ref")
	}
}

func TestLog_WithNote(t *testing.T) {
	requireGit(t)
	dir, headSHA := makeTempGitRepo(t)
	ref := RefForType(TypeDecision)
	if err := AddNote(dir, ref, headSHA, "log-test-body"); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	entries, err := Log(dir, ref, 10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Log len = %d, want 1", len(entries))
	}
	if entries[0].CommitSHA != headSHA || entries[0].Body != "log-test-body" {
		t.Errorf("Log[0] = %+v", entries[0])
	}
	// limit
	entries2, err := Log(dir, ref, 0)
	if err != nil {
		t.Fatalf("Log limit 0: %v", err)
	}
	if len(entries2) != 0 {
		t.Errorf("Log(limit=0) len = %d, want 0", len(entries2))
	}
}

func TestDiff_Validation(t *testing.T) {
	requireGit(t)
	ref := RefForType(TypeDecision)
	_, err := Diff("", ref, "main", "HEAD")
	if err == nil {
		t.Error("expected error for empty repoPath")
	}
	_, err = Diff("/tmp", "", "main", "HEAD")
	if err == nil {
		t.Error("expected error for empty ref")
	}
	_, err = Diff("/tmp", ref, "", "HEAD")
	if err == nil {
		t.Error("expected error for empty base")
	}
	_, err = Diff("/tmp", ref, "HEAD", "")
	if err == nil {
		t.Error("expected error for empty head")
	}
}

func TestDiff_WithNote(t *testing.T) {
	requireGit(t)
	dir, firstSHA := makeTempGitRepo(t)
	// Second commit
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "second")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	headSHA := strings.TrimSpace(string(out))
	ref := RefForType(TypeDecision)
	if err := AddNote(dir, ref, headSHA, "diff-body"); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	// firstSHA..headSHA = one commit (head)
	entries, err := Diff(dir, ref, firstSHA, headSHA)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Diff len = %d, want 1", len(entries))
	}
	if entries[0].CommitSHA != headSHA || entries[0].Body != "diff-body" {
		t.Errorf("Diff[0] = %+v", entries[0])
	}
}

func TestDiff_EmptyRange(t *testing.T) {
	requireGit(t)
	dir, _ := makeTempGitRepo(t)
	ref := RefForType(TypeDecision)
	// HEAD..HEAD is empty
	entries, err := Diff(dir, ref, "HEAD", "HEAD")
	if err != nil {
		t.Fatalf("Diff HEAD..HEAD: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Diff(HEAD..HEAD) len = %d, want 0", len(entries))
	}
}

