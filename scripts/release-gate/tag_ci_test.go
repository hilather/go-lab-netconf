package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseGateAcceptsMatchingTagPush(t *testing.T) {
	mark := installGH(t, oneRun(7, "completed", "success", "push", "v1.2.3"), viewGreen())
	if err := requireGreenCI(); err != nil {
		t.Fatal(err)
	}
	assertHeadBranchRequested(t, mark)
}

func TestReleaseGateRejectsNonTagCI(t *testing.T) {
	mark := installGH(t, oneRun(42, "completed", "success", "pull_request", "v1.2.3"), viewGreen())
	err := requireGreenCI()
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
	err := requireGreenCI()
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
	err := requireGreenCI()
	if err == nil || !strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
		t.Fatalf("release gate = %v, want pending", err)
	}
	assertHeadBranchRequested(t, mark)
}

func TestReleaseTagResolvesRefNameWhenRefIsBranch(t *testing.T) {
	installGH(t, oneRun(7, "completed", "success", "push", "v1.2.3"), viewGreen())
	t.Setenv("GITHUB_REF", "refs/heads/main")
	t.Setenv("GITHUB_REF_NAME", "v1.2.3")
	if err := requireGreenCI(); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseTagRejectsEmptyName(t *testing.T) {
	installGH(t, oneRun(7, "completed", "success", "push", "v1.2.3"), viewGreen())
	t.Setenv("GITHUB_REF", "refs/heads/main")
	t.Setenv("GITHUB_REF_NAME", "")
	err := requireGreenCI()
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
	script := filepath.Join(dir, "gh")
	body := fmt.Sprintf(`#!/bin/sh
for a in "$@"; do
  case "$a" in
  *headBranch*) touch %q ;;
  esac
done
case "$1 $2" in
"run list")
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
`, mark, strings.TrimSpace(listJSON), viewBody)
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
