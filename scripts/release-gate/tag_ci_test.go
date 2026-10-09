package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseGateAcceptsMatchingTagPush(t *testing.T) {
	mark := installGH(t, oneRun(7, "completed", "success", "push", "v1.2.3"), viewGreen())
	if err := requireGreenCI("", ""); err != nil {
		t.Fatal(err)
	}
	assertHeadBranchRequested(t, mark)
}

func TestReleaseGateRejectsNonTagCI(t *testing.T) {
	mark := installGH(t, oneRun(42, "completed", "success", "pull_request", "v1.2.3"), viewGreen())
	err := requireGreenCI("", "")
	if err == nil || !strings.Contains(err.Error(), "no matching run") {
		t.Fatalf("release gate = %v, want no matching run", err)
	}
	assertHeadBranchRequested(t, mark)
}

func TestReleaseGateRejectsGreenMainWithRedTag(t *testing.T) {
	list := `[
{"databaseId":10,"conclusion":"success","status":"completed","headSha":"abc123","event":"push","headBranch":"main"},
{"databaseId":20,"conclusion":"failure","status":"completed","headSha":"abc123","event":"push","headBranch":"v1.2.3"}
]`
	mark := installGH(t, list, viewByID())
	err := requireGreenCI("", "")
	if err == nil || strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
		t.Fatalf("release gate = %v, want a red-job error", err)
	}
	assertHeadBranchRequested(t, mark)
}

func TestReleaseGatePendingNewerTagRun(t *testing.T) {
	list := `[
{"databaseId":10,"conclusion":"success","status":"completed","headSha":"abc123","event":"push","headBranch":"v1.2.3"},
{"databaseId":30,"conclusion":"","status":"in_progress","headSha":"abc123","event":"push","headBranch":"v1.2.3"}
]`
	mark := installGH(t, list, viewGreen())
	err := requireGreenCI("", "")
	if err == nil || !strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
		t.Fatalf("release gate = %v, want pending", err)
	}
	assertHeadBranchRequested(t, mark)
}

func TestReleaseTagResolvesRefNameWhenRefIsBranch(t *testing.T) {
	installGH(t, oneRun(7, "completed", "success", "push", "v1.2.3"), viewGreen())
	t.Setenv("GITHUB_REF", "refs/heads/main")
	t.Setenv("GITHUB_REF_NAME", "v1.2.3")
	if err := requireGreenCI("", ""); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseTagRejectsEmptyName(t *testing.T) {
	installGH(t, oneRun(7, "completed", "success", "push", "v1.2.3"), viewGreen())
	t.Setenv("GITHUB_REF", "refs/heads/main")
	t.Setenv("GITHUB_REF_NAME", "")
	err := requireGreenCI("", "")
	if err == nil {
		t.Fatal("empty tag was accepted")
	}
	if strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
		t.Fatalf("empty tag error = %v, must not look like a run-selection error", err)
	}
}

func TestReleaseWorkflowDoesNotInterpolateTagIntoShell(t *testing.T) {
	for _, name := range []string{"release.yml", "ci.yml"} {
		text := readWorkflow(t, name)
		if lines := runInterpolationLines(text); len(lines) > 0 {
			t.Errorf("%s interpolates ${{ into run: at lines %v", name, lines)
		}
	}
}

func TestRunInterpolationDetector(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want bool
	}{
		{
			name: "inline run",
			yaml: "      - name: x\n        run: echo ${{ github.ref }}\n",
			want: true,
		},
		{
			name: "block run",
			yaml: "      - name: x\n        run: |\n          echo ${{ github.ref }}\n",
			want: true,
		},
		{
			name: "with block",
			yaml: "      - name: x\n        with:\n          ref: ${{ github.ref }}\n",
			want: false,
		},
		{
			name: "concurrency",
			yaml: "concurrency:\n  group: ${{ github.ref }}\n",
			want: false,
		},
		{
			name: "block run blank line",
			yaml: "      - name: x\n        run: |\n          echo ok\n\n          echo ${{ github.ref }}\n",
			want: true,
		},
		{
			name: "env row",
			yaml: "      - name: x\n        env:\n          REF: ${{ github.ref }}\n        run: echo \"$REF\"\n",
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := len(runInterpolationLines(tc.yaml)) > 0
			if got != tc.want {
				t.Fatalf("hit=%v want %v lines %v", got, tc.want, runInterpolationLines(tc.yaml))
			}
		})
	}
}

func runInterpolationLines(text string) []int {
	lines := strings.Split(text, "\n")
	var hits []int
	inRun := false
	runIndent := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := 0
		for indent < len(line) && line[indent] == ' ' {
			indent++
		}
		if inRun && indent <= runIndent {
			inRun = false
		}
		if inRun {
			if strings.Contains(line, "${{") {
				hits = append(hits, i+1)
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "run:") {
			continue
		}
		inRun = true
		runIndent = indent
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "run:"))
		switch rest {
		case "", "|", "|-", ">", ">-", "|+", ">+":
		default:
			if strings.Contains(rest, "${{") {
				hits = append(hits, i+1)
			}
		}
	}
	return hits
}

func installGH(t *testing.T, listJSON, viewBody string) string {
	t.Helper()
	dir := t.TempDir()
	mark := filepath.Join(dir, "headbranch")
	argsPath := filepath.Join(dir, "args")
	script := filepath.Join(dir, "gh")
	body := fmt.Sprintf(`#!/bin/sh
for a in "$@"; do
  case "$a" in
  *headBranch*) touch %q ;;
  esac
done
case "$1 $2" in
"run list")
  printf '%%s\n' "$@" > %q
  cat <<'EOF'
%s
EOF
  ;;
"run view")
%s
  ;;
*)
  echo "unexpected: $*" >&2
  exit 2
  ;;
esac
`, mark, argsPath, strings.TrimSpace(listJSON), viewBody)
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GITHUB_SHA", "abc123")
	t.Setenv("GITHUB_REF", "refs/tags/v1.2.3")
	t.Setenv("GITHUB_REF_NAME", "v1.2.3")
	t.Setenv("GH_TOKEN", "test")
	return mark
}

func assertHeadBranchRequested(t *testing.T, mark string) {
	t.Helper()
	if _, err := os.Stat(mark); err != nil {
		t.Fatal("gh run list omitted headBranch")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(mark), "args"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "--limit=200") {
		t.Fatalf("gh run list args missing --limit=200:\n%s", body)
	}
}

func oneRun(id int, status, conclusion, event, branch string) string {
	return fmt.Sprintf(`[{"databaseId":%d,"conclusion":%q,"status":%q,"headSha":"abc123","event":%q,"headBranch":%q}]`,
		id, conclusion, status, event, branch)
}

func viewGreen() string {
	return "  cat <<'EOF'\n" + jobsJSON("") + "EOF\n"
}

func viewByID() string {
	return "  case \"$3\" in\n  20)\n    cat <<'EOF'\n" + jobsJSON("failure") + "EOF\n    ;;\n  *)\n    cat <<'EOF'\n" + jobsJSON("") + "EOF\n    ;;\n  esac\n"
}

func jobsJSON(unitConclusion string) string {
	names := []string{
		"format", "lint", "unit", "race", "fuzz-smoke", "documentation",
		"config-compat", "changelog", "generated-file", "parity", "security-scan",
		"container-test", "web",
	}
	var b strings.Builder
	b.WriteString("{\"jobs\":[\n")
	for i, name := range names {
		conclusion := "success"
		if name == "unit" && unitConclusion != "" {
			conclusion = unitConclusion
		}
		fmt.Fprintf(&b, "  {\"name\":%q,\"conclusion\":%q}", name, conclusion)
		if i < len(names)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString("]}\n")
	return b.String()
}

func readWorkflow(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		p := filepath.Join(dir, ".github", "workflows", name)
		if b, err := os.ReadFile(p); err == nil {
			return string(b)
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("%s not found", name)
	return ""
}

const (
	tagSHA    = "1111111111111111111111111111111111abcdef"
	branchSHA = "2222222222222222222222222222222222222222"
)

// dispatchEnv is what the gate sees on workflow_dispatch from main: GitHub
// ignores the workflow's step env for GITHUB_*, so they name the branch.
func dispatchEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GITHUB_REF", "refs/heads/main")
	t.Setenv("GITHUB_REF_NAME", "main")
	t.Setenv("GITHUB_SHA", branchSHA)
}

func tagRunAt(sha string) string {
	return `[{"databaseId":7,"conclusion":"success","status":"completed","headSha":"` + sha + `","event":"push","headBranch":"v1.2.3"}]`
}

func listArgs(t *testing.T, mark string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(filepath.Dir(mark), "args"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// mainAndTagRuns has main's green push at the branch head and the tag's
// push at the tag commit.
func mainAndTagRuns() string {
	return `[{"databaseId":5,"conclusion":"success","status":"completed","headSha":"` + branchSHA + `","event":"push","headBranch":"main"},` +
		`{"databaseId":7,"conclusion":"success","status":"completed","headSha":"` + tagSHA + `","event":"push","headBranch":"v1.2.3"}]`
}

// Before -tag/-sha, a dispatch from main resolved the tag "main" and
// accepted main's own green push run.
func TestReleaseGateDispatchEnvWithoutFlagsFails(t *testing.T) {
	installGH(t, mainAndTagRuns(), viewGreen())
	dispatchEnv(t)
	err := requireGreenCI("", "")
	if err == nil {
		t.Fatal("dispatch env without -tag/-sha accepted main's push run")
	}
	if strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
		t.Fatalf("branch ref looks retryable: %v", err)
	}
}

func TestReleaseGateExplicitFlagsIgnoreMainRun(t *testing.T) {
	mark := installGH(t, `[{"databaseId":5,"conclusion":"success","status":"completed","headSha":"`+branchSHA+`","event":"push","headBranch":"main"}]`, viewGreen())
	dispatchEnv(t)
	err := requireGreenCI("v1.2.3", tagSHA)
	if err == nil || !strings.Contains(err.Error(), "no matching run") {
		t.Fatalf("err=%v", err)
	}
	if args := listArgs(t, mark); !strings.Contains(args, "--commit="+tagSHA) {
		t.Fatalf("gh args:\n%s", args)
	}
}

func TestReleaseGateExplicitTagAndSHA(t *testing.T) {
	for _, tag := range []string{"v1.2.3", "refs/tags/v1.2.3", " v1.2.3 "} {
		t.Run(tag, func(t *testing.T) {
			mark := installGH(t, tagRunAt(tagSHA), viewGreen())
			dispatchEnv(t)
			var errb bytes.Buffer
			if code := run([]string{"-require-ci", "-tag", tag, "-sha", tagSHA}, &errb); code != 0 {
				t.Fatalf("code %d: %s", code, errb.String())
			}
			args := listArgs(t, mark)
			if !strings.Contains(args, "--commit="+tagSHA) || strings.Contains(args, branchSHA) {
				t.Fatalf("gh args:\n%s", args)
			}
		})
	}
}

func TestReleaseGateTagWithoutSHAUsesHEAD(t *testing.T) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Skipf("no git HEAD: %v", err)
	}
	head := strings.TrimSpace(string(out))
	mark := installGH(t, tagRunAt(head), viewGreen())
	dispatchEnv(t)
	if err := requireGreenCI("v1.2.3", ""); err != nil {
		t.Fatal(err)
	}
	if args := listArgs(t, mark); !strings.Contains(args, "--commit="+head) || strings.Contains(args, branchSHA) {
		t.Fatalf("gh args:\n%s", args)
	}
}

func TestReleaseGateExplicitFlagsRejectBadValues(t *testing.T) {
	cases := []struct{ tag, sha, want string }{
		{"main", tagSHA, "-tag"},
		{"v1.2.3;rm", tagSHA, "-tag"},
		{"v1.2.3", "abc", "-sha"},
		{"v1.2.3", strings.ToUpper(tagSHA), "-sha"},
		{"v1.2.3", tagSHA + ";rm", "-sha"},
		{"v1.2.3", "-h", "-sha"},
		{"v1.2.3-pending;", tagSHA, "-tag"},
		{"no matching run", tagSHA, "-tag"},
		{"v1.2.3", "pending", "-sha"},
		{"v1.2.3", "no matching run", "-sha"},
	}
	for _, tc := range cases {
		t.Run(tc.tag+"/"+tc.sha, func(t *testing.T) {
			installGH(t, tagRunAt(tagSHA), "  echo 'gh run view must not run' >&2\n  exit 9\n")
			dispatchEnv(t)
			err := requireGreenCI(tc.tag, tc.sha)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v", err)
			}
			if strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
				t.Fatalf("bad flag looks retryable: %v", err)
			}
		})
	}
}

func TestRunUsage(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"-notes-only", "-require-ci"},
		{"-notes-only", "-notes", "x.md", "-tag", "v1.2.3"},
		{"-notes-only", "-notes", "x.md", "-sha", tagSHA},
		{"-require-ci", "-notes", "x.md"},
		{"-require-ci", "extra"},
	} {
		var errb bytes.Buffer
		if code := run(args, &errb); code != 2 {
			t.Fatalf("%q: code %d", args, code)
		}
	}
}

// GitHub ignores step env that sets GITHUB_*, so the re-gate must pass the
// tag and commit to release-gate explicitly.
func TestReleaseWorkflowPassesTagAndSHA(t *testing.T) {
	rel := readWorkflow(t, "release.yml")
	for _, bad := range []string{"GITHUB_SHA:", "GITHUB_REF:", "GITHUB_REF_NAME:"} {
		if strings.Contains(rel, bad) {
			t.Errorf("release.yml sets %s in env; GitHub ignores it", strings.TrimSuffix(bad, ":"))
		}
	}
	for _, want := range []string{
		"RELEASE_TAG: ${{ steps.gate.outputs.ref }}",
		"RELEASE_SHA: ${{ steps.rev.outputs.sha }}",
		`go run ./scripts/release-gate -require-ci -tag "$RELEASE_TAG" -sha "$RELEASE_SHA"`,
	} {
		if !strings.Contains(rel, want) {
			t.Errorf("release.yml missing %q", want)
		}
	}
}

// The env-path tag error is fixed text, so a ref whose name contains the
// workflow's retry words is not retried.
func TestEnvTagErrorDoesNotEchoRef(t *testing.T) {
	for _, name := range []string{"pending", "no matching run", "x-pending"} {
		t.Run(name, func(t *testing.T) {
			installGH(t, mainAndTagRuns(), viewGreen())
			dispatchEnv(t)
			t.Setenv("GITHUB_REF_NAME", name)
			err := requireGreenCI("", "")
			if err == nil {
				t.Fatal("branch name accepted as a tag")
			}
			if strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
				t.Fatalf("env tag error looks retryable: %v", err)
			}
		})
	}
}
