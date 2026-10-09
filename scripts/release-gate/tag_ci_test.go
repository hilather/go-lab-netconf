package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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
	if err == nil || !retryable(err) || !strings.Contains(err.Error(), "no matching run") {
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
	if err == nil || retryable(err) || strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
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
	if err == nil || !retryable(err) || !strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
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
	if err == nil || retryable(err) {
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

// viewJobs builds a gh run view jobs document. A required name listed in
// conclusions is emitted once per conclusion, in slice order. Every other
// required name is one success, except skip, which is omitted. A conclusions
// key that is not a required name is appended.
func viewJobs(conclusions map[string][]string, skip string) string {
	required := make(map[string]struct{}, len(requiredCIJobs))
	for _, name := range requiredCIJobs {
		required[name] = struct{}{}
	}
	type one struct{ name, conclusion string }
	var jobs []one
	for _, name := range requiredCIJobs {
		if name == skip {
			continue
		}
		cs, ok := conclusions[name]
		if !ok {
			cs = []string{"success"}
		}
		for _, c := range cs {
			jobs = append(jobs, one{name, c})
		}
	}
	var extras []string
	for name := range conclusions {
		if _, ok := required[name]; !ok {
			extras = append(extras, name)
		}
	}
	sort.Strings(extras)
	for _, name := range extras {
		for _, c := range conclusions[name] {
			jobs = append(jobs, one{name, c})
		}
	}
	var b strings.Builder
	b.WriteString(`{"jobs":[`)
	for i, j := range jobs {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"name":%q,"conclusion":%q}`, j.name, j.conclusion)
	}
	b.WriteString("]}")
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
	if err == nil || retryable(err) {
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
	if err == nil || !retryable(err) || !strings.Contains(err.Error(), "no matching run") {
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
			if err == nil || retryable(err) || !strings.Contains(err.Error(), tc.want) {
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

// releaseGroupExpr is the workflow concurrency group value, without the
// "group: " prefix.
const releaseGroupExpr = "release-${{ github.workflow }}-${{ startsWith(github.event.inputs.ref || github.ref, 'refs/') && (github.event.inputs.ref || github.ref) || format('refs/tags/{0}', github.event.inputs.ref) }}"

// checkReleaseWorkflowConcurrency is the concurrency authority for release.yml.
// The node walk does not see # comments. Callers keep a raw absent-list so
// the old group string still fails when it appears only in a comment.
func checkReleaseWorkflowConcurrency(t *testing.T, rel string) {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(rel), &doc); err != nil {
		t.Fatalf("release.yml: %v", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		t.Fatalf("release.yml document kind=%d content=%d, want one document root", doc.Kind, len(doc.Content))
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		t.Fatalf("release.yml root kind=%d, want a mapping", root.Kind)
	}

	var conc, jobs []*yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		switch root.Content[i].Value {
		case "concurrency":
			conc = append(conc, root.Content[i+1])
		case "jobs":
			jobs = append(jobs, root.Content[i+1])
		}
	}
	if len(conc) != 1 {
		parsed := make([]string, len(conc))
		for i, n := range conc {
			parsed[i] = formatYAMLValue(n)
		}
		t.Errorf("root concurrency keys = %d, want 1; parsed [%s]", len(conc), strings.Join(parsed, "; "))
	} else {
		checkConcurrencyBlock(t, conc[0])
	}
	if len(jobs) == 0 {
		t.Errorf("root jobs keys = 0, want a jobs mapping")
	}
	for _, n := range jobs {
		checkJobsHaveNoConcurrency(t, n)
	}
}

func checkConcurrencyBlock(t *testing.T, n *yaml.Node) {
	t.Helper()
	if n.Kind != yaml.MappingNode {
		t.Errorf("concurrency %s, want a mapping with group and cancel-in-progress", formatYAMLValue(n))
		return
	}
	pairs := len(n.Content) / 2
	var groups, cancels []*yaml.Node
	keys := make([]string, 0, pairs)
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		keys = append(keys, k.Value)
		switch k.Value {
		case "group":
			groups = append(groups, v)
		case "cancel-in-progress":
			cancels = append(cancels, v)
		}
	}
	if pairs != 2 || len(groups) != 1 || len(cancels) != 1 || len(n.Content) != 4 {
		t.Errorf("concurrency pairs = %d keys = %q parsed %s, want exactly group and cancel-in-progress", pairs, keys, formatYAMLValue(n))
	}
	if len(groups) == 1 && groups[0].Value != releaseGroupExpr {
		t.Errorf("concurrency group = %q, want %q", groups[0].Value, releaseGroupExpr)
	}
	if len(cancels) == 1 {
		tag := cancels[0].ShortTag()
		if tag != "!!bool" || cancels[0].Value != "false" {
			t.Errorf("concurrency cancel-in-progress tag=%s value=%q, want !!bool \"false\"", tag, cancels[0].Value)
		}
	}
}

func checkJobsHaveNoConcurrency(t *testing.T, jobs *yaml.Node) {
	t.Helper()
	if jobs.Kind != yaml.MappingNode {
		t.Errorf("jobs %s, want a mapping of job names", formatYAMLValue(jobs))
		return
	}
	for i := 0; i+1 < len(jobs.Content); i += 2 {
		name, job := jobs.Content[i].Value, jobs.Content[i+1]
		if job.Kind != yaml.MappingNode {
			t.Errorf("job %s %s, want a mapping", name, formatYAMLValue(job))
			continue
		}
		for j := 0; j+1 < len(job.Content); j += 2 {
			if job.Content[j].Value != "concurrency" {
				continue
			}
			t.Errorf("job %s has a concurrency key: %s", name, formatYAMLValue(job.Content[j+1]))
		}
	}
}

func formatYAMLValue(n *yaml.Node) string {
	if n == nil {
		return "nil"
	}
	if n.Kind != yaml.MappingNode {
		return fmt.Sprintf("tag=%s value=%q", n.ShortTag(), n.Value)
	}
	parts := make([]string, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		parts = append(parts, fmt.Sprintf("%s={tag=%s value=%q}", k.Value, v.ShortTag(), v.Value))
	}
	return fmt.Sprintf("%d pairs [%s]", len(n.Content)/2, strings.Join(parts, " "))
}

// GitHub ignores step env that sets GITHUB_*, so the re-gate must pass the
// tag and peeled commit to release-gate explicitly. Checkout is only the
// canonical tag, and the workflow retries only exit 75. publish-image
// builds only the commit tag-gate approved.
func TestWorkflowContract(t *testing.T) {
	rel := readWorkflow(t, "release.yml")
	for _, bad := range []string{"GITHUB_SHA:", "GITHUB_REF:", "GITHUB_REF_NAME:"} {
		if strings.Contains(rel, bad) {
			t.Errorf("release.yml sets %s in env; GitHub ignores it", strings.TrimSuffix(bad, ":"))
		}
	}
	for _, want := range []string{
		"RELEASE_TAG: ${{ steps.rev.outputs.ref }}",
		"RELEASE_SHA: ${{ steps.rev.outputs.sha }}",
		`"$RUNNER_TEMP/release-gate" -require-ci -tag "$RELEASE_TAG" -sha "$RELEASE_SHA"`,
		`go build -o "$RUNNER_TEMP/release-gate"`,
		"refs/tags/${ref}^{commit}",
	} {
		if !strings.Contains(rel, want) {
			t.Errorf("release.yml missing %q", want)
		}
	}
	// Positive group / cancel-in-progress substring checks are not used.
	// A # comment satisfies them while the live mapping is something else.
	checkReleaseWorkflowConcurrency(t, rel)
	if got := strings.Count(rel, "re='"+releaseTagPatternSrc+"'"); got != 4 {
		t.Fatalf("shell tag pattern count = %d, want 4", got)
	}
	if got := strings.Count(rel, `refs/tags/*) ref="${ref#refs/tags/}" ;;`); got != 4 {
		t.Fatalf("refs/tags strip count = %d, want 4", got)
	}
	if strings.Contains(rel, `^v[0-9A-Za-z.+-]+$`) {
		t.Fatal("old tag pattern still present")
	}
	if strings.Contains(rel, "steps.gate.outputs") {
		t.Fatal("release.yml still reads steps.gate")
	}
	canonStep := "\n      - name: Canonicalize release ref\n"
	checkoutStep := "\n      - uses: actions/checkout@"
	canon := strings.Index(rel, canonStep)
	checkout := strings.Index(rel, checkoutStep)
	if canon < 0 || checkout < 0 || canon > checkout {
		t.Fatal("canonicalize step must precede checkout")
	}
	if got := strings.Count(rel, "ref: refs/tags/${{ steps.tag.outputs.ref }}"); got != 2 {
		t.Fatalf("canonical checkout ref count = %d, want 2", got)
	}
	checkouts := strings.Split(rel, "uses: actions/checkout@")
	if len(checkouts) != 3 {
		t.Fatalf("actions/checkout steps = %d, want 2", len(checkouts)-1)
	}
	for i, chunk := range checkouts[1:] {
		if nl := strings.Index(chunk, "\n      - "); nl >= 0 {
			chunk = chunk[:nl]
		}
		cred := strings.Index(chunk, "persist-credentials: false")
		refAt := strings.Index(chunk, "ref: refs/tags/${{ steps.tag.outputs.ref }}")
		if cred < 0 || refAt < 0 || cred > refAt {
			t.Fatalf("checkout %d: persist-credentials: false must sit before the tag ref", i+1)
		}
	}
	if got := strings.Count(rel, "persist-credentials: false"); got != 2 {
		t.Fatalf("persist-credentials: false count = %d, want 2", got)
	}
	if strings.Contains(rel, "persist-credentials: true") {
		t.Fatal("release.yml sets persist-credentials: true")
	}
	// Raw substring, including comments. The old group string fails here
	// even when it is only a # comment. The yaml.v3 walk above is what
	// accepts or rejects the live concurrency mapping.
	for _, absent := range []string{
		"ref: ${{ github.event.inputs.ref || github.ref }}",
		"ref: ${{ github.ref }}",
		"github.ref_name",
		"*pending*",
		`*"no matching run"*`,
		`case "$out"`,
		"COMMIT=${{ github.sha }}",
		"group: release-${{ github.workflow }}-${{ github.event.inputs.ref || github.ref }}",
	} {
		if strings.Contains(rel, absent) {
			t.Errorf("release.yml contains %q", absent)
		}
	}
	wantStatus := fmt.Sprintf(`[ "$status" -eq %d ]`, exitRetryable)
	if !strings.Contains(rel, wantStatus) {
		t.Fatalf("workflow retry status drifted from exit %d", exitRetryable)
	}
	if !strings.Contains(rel, "github.event_name == 'push'") || !strings.Contains(rel, "startsWith(github.ref, 'refs/tags/v')") {
		t.Fatal("publish-image if drifted")
	}

	gateKey := "\n  tag-gate:\n"
	pubKey := "\n  publish-image:\n"
	gateAt := strings.Index(rel, gateKey)
	pubAt := strings.Index(rel, pubKey)
	if gateAt < 0 || pubAt < 0 || gateAt > pubAt {
		t.Fatal("job keys missing")
	}
	tagGate := rel[gateAt:pubAt]
	publish := rel[pubAt:]
	gateCanon := strings.Index(tagGate, canonStep)
	gateCheckout := strings.Index(tagGate, checkoutStep)
	if gateCanon < 0 || gateCheckout < 0 || gateCanon > gateCheckout {
		t.Fatal("tag-gate: canonicalize step must precede checkout")
	}
	headMismatch := `if [ "$head" != "$sha" ]; then
            echo "HEAD ${head} is not the peeled commit ${sha} of ${ref}" >&2
            exit 1
          fi`
	gatedMismatch := `if [ -z "$GATED_SHA" ] || [ "$ref" != "$GATED_REF" ] || [ "$sha" != "$GATED_SHA" ]; then
            echo "tag ${ref} commit ${sha} is not tag-gate ref ${GATED_REF} commit ${GATED_SHA}" >&2
            exit 1
          fi`
	outKey := "\n    outputs:\n      sha: ${{ steps.rev.outputs.sha }}\n      ref: ${{ steps.rev.outputs.ref }}\n"
	stepsKey := "\n    steps:\n"
	outAt := strings.Index(tagGate, outKey)
	stepsAt := strings.Index(tagGate, stepsKey)
	if outAt < 0 || stepsAt < 0 || outAt > stepsAt {
		t.Fatal("tag-gate: outputs must expose steps.rev sha and ref")
	}
	if !strings.Contains(tagGate, "refs/tags/${ref}^{commit}") || !strings.Contains(tagGate, headMismatch) {
		t.Fatal("tag-gate: missing peel")
	}
	if !strings.Contains(publish, headMismatch) {
		t.Fatal("publish-image: HEAD mismatch check is missing its exit 1")
	}
	publishOrder := []string{
		canonStep,
		checkoutStep,
		"persist-credentials: false",
		"ref: refs/tags/${{ steps.tag.outputs.ref }}",
		"GATED_SHA: ${{ needs.tag-gate.outputs.sha }}",
		"GATED_REF: ${{ needs.tag-gate.outputs.ref }}",
		"refs/tags/${ref}^{commit}",
		headMismatch,
		gatedMismatch,
		"VERSION=${{ steps.tag.outputs.ref }}",
		"COMMIT=${{ steps.rev.outputs.sha }}",
		"RELEASE_REF: ${{ steps.tag.outputs.ref }}",
	}
	from := 0
	for _, sub := range publishOrder {
		i := strings.Index(publish[from:], sub)
		if i < 0 {
			t.Fatalf("publish-image missing %q after previous hit", sub)
		}
		from += i + len(sub)
	}
}

// ciJobsNotRequired lists CI job display names the release gate does not
// require, each with a reason. Any CI job not gated on release must be
// listed here with a reason.
var ciJobsNotRequired = map[string]string{}

// TestRequiredCIJobsMatchCIWorkflow fails unless ci.yml's display names,
// minus ciJobsNotRequired, are exactly requiredCIJobs. A matrix multiplies
// GitHub check names, so strategy.matrix is rejected. Display name is the
// job's name, or the job id when name is unset.
func TestRequiredCIJobsMatchCIWorkflow(t *testing.T) {
	required := make(map[string]int, len(requiredCIJobs))
	for _, name := range requiredCIJobs {
		required[name]++
		if required[name] > 1 {
			t.Errorf("requiredCIJobs lists %q more than once", name)
		}
	}

	// scripts/release-gate -> repo root.
	body, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Jobs map[string]struct {
			Name     string         `yaml:"name"`
			Strategy map[string]any `yaml:"strategy"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("ci.yml: %v", err)
	}
	if len(doc.Jobs) == 0 {
		t.Fatal("ci.yml has no jobs")
	}

	ids := make([]string, 0, len(doc.Jobs))
	for id := range doc.Jobs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	displayOf := make(map[string]string, len(ids))
	for _, id := range ids {
		job := doc.Jobs[id]
		if _, ok := job.Strategy["matrix"]; ok {
			t.Errorf("ci.yml job %q has strategy.matrix; a matrix multiplies check names", id)
		}
		name := job.Name
		if name == "" {
			name = id
		}
		if prev, ok := displayOf[name]; ok {
			t.Errorf("ci.yml jobs %q and %q share display name %q", prev, id, name)
			continue
		}
		displayOf[name] = id
	}

	exceptNames := make([]string, 0, len(ciJobsNotRequired))
	for name := range ciJobsNotRequired {
		exceptNames = append(exceptNames, name)
	}
	sort.Strings(exceptNames)
	for _, name := range exceptNames {
		if _, ok := displayOf[name]; !ok {
			t.Errorf("ciJobsNotRequired names %q, which is not a ci.yml display name", name)
		}
		if required[name] > 0 {
			t.Errorf("ciJobsNotRequired names %q, which is also in requiredCIJobs", name)
		}
	}

	var onlyCI, onlyRequired []string
	for name := range displayOf {
		if _, skip := ciJobsNotRequired[name]; skip {
			continue
		}
		if required[name] == 0 {
			onlyCI = append(onlyCI, name)
		}
	}
	for name := range required {
		if _, skip := ciJobsNotRequired[name]; skip {
			onlyRequired = append(onlyRequired, name)
			continue
		}
		if _, ok := displayOf[name]; !ok {
			onlyRequired = append(onlyRequired, name)
		}
	}
	if len(onlyCI) > 0 || len(onlyRequired) > 0 {
		sort.Strings(onlyCI)
		sort.Strings(onlyRequired)
		t.Errorf("ci.yml display names minus ciJobsNotRequired != requiredCIJobs\n  ci.yml only: %q\n  requiredCIJobs only: %q", onlyCI, onlyRequired)
	}
}

// The env-path tag error is fixed text and does not echo the ref. The
// retry signal is exit 75, and this error is not one. run([]string{"-require-ci"})
// returns 1 for GITHUB_REF_NAME of pending, no matching run, and x-pending.
func TestEnvTagErrorDoesNotEchoRef(t *testing.T) {
	for _, name := range []string{"pending", "no matching run", "x-pending"} {
		t.Run(name, func(t *testing.T) {
			installGH(t, mainAndTagRuns(), viewGreen())
			dispatchEnv(t)
			t.Setenv("GITHUB_REF_NAME", name)
			err := requireGreenCI("", "")
			if err == nil || retryable(err) {
				t.Fatalf("branch name accepted or retryable: %v", err)
			}
			if strings.Contains(err.Error(), "pending") || strings.Contains(err.Error(), "no matching run") {
				t.Fatalf("env tag error looks retryable: %v", err)
			}
			var errb bytes.Buffer
			if code := run([]string{"-require-ci"}, &errb); code != 1 {
				t.Fatalf("code %d\n%s", code, errb.String())
			}
		})
	}
}

func TestReleaseTagPattern(t *testing.T) {
	accepts := []struct {
		raw  string
		want string
	}{
		{"v1.2.3", "v1.2.3"},
		{"v1.2.3-rc.1", "v1.2.3-rc.1"},
		{"v1.0.0-pending", "v1.0.0-pending"},
		{"v01.2.3", "v01.2.3"},
		{"refs/tags/v1.2.3", "v1.2.3"},
		{" v1.2.3 ", "v1.2.3"},
	}
	for _, tc := range accepts {
		t.Run("flag/"+tc.raw, func(t *testing.T) {
			got, err := flagTag(tc.raw)
			if err != nil || got != tc.want {
				t.Fatalf("flagTag(%q)=%q %v, want %q", tc.raw, got, err, tc.want)
			}
		})
	}
	t.Run("env ref prefix", func(t *testing.T) {
		t.Setenv("GITHUB_REF", "refs/tags/v1.2.3")
		t.Setenv("GITHUB_REF_NAME", "ignored")
		got, err := releaseTag()
		if err != nil || got != "v1.2.3" {
			t.Fatalf("releaseTag=%q %v", got, err)
		}
	})
	for _, name := range []string{"v1.2.3", "v1.2.3-rc.1", "v1.0.0-pending", "v01.2.3"} {
		t.Run("env name/"+name, func(t *testing.T) {
			t.Setenv("GITHUB_REF", "refs/heads/main")
			t.Setenv("GITHUB_REF_NAME", name)
			got, err := releaseTag()
			if err != nil || got != name {
				t.Fatalf("releaseTag=%q %v", got, err)
			}
		})
	}
	t.Run("env name padded", func(t *testing.T) {
		t.Setenv("GITHUB_REF", "")
		t.Setenv("GITHUB_REF_NAME", " v1.2.3 ")
		got, err := releaseTag()
		if err != nil || got != "v1.2.3" {
			t.Fatalf("releaseTag=%q %v", got, err)
		}
	})
	for _, raw := range []string{"vpending", "v1", "v1.2", "v1.2.3.4", "v1.2.3+meta", "main", ""} {
		t.Run("flag reject/"+raw, func(t *testing.T) {
			_, err := flagTag(raw)
			if err == nil || retryable(err) {
				t.Fatalf("flagTag(%q) err=%v", raw, err)
			}
		})
		t.Run("env reject/"+raw, func(t *testing.T) {
			t.Setenv("GITHUB_REF", "refs/heads/main")
			t.Setenv("GITHUB_REF_NAME", raw)
			_, err := releaseTag()
			if err == nil || retryable(err) {
				t.Fatalf("releaseTag(%q) err=%v", raw, err)
			}
		})
	}
}

func TestExitCodes(t *testing.T) {
	t.Run("in progress", func(t *testing.T) {
		installGH(t, listJSON(30, "in_progress", "", "v1.2.3", tagSHA), "  echo 'view must not run' >&2\n  exit 9\n")
		var errb bytes.Buffer
		code := run([]string{"-require-ci", "-tag", "v1.2.3", "-sha", tagSHA}, &errb)
		want := fmt.Sprintf("release-gate: %s CI run %d for tag %s\n", pendingPrefix, 30, "v1.2.3")
		if code != exitRetryable || errb.String() != want || !strings.HasPrefix(errb.String(), "release-gate: pending ") {
			t.Fatalf("code %d\n%s", code, errb.String())
		}
	})
	t.Run("no match", func(t *testing.T) {
		installGH(t, "[]", "  echo 'view must not run' >&2\n  exit 9\n")
		var errb bytes.Buffer
		code := run([]string{"-require-ci", "-tag", "v1.2.3", "-sha", tagSHA}, &errb)
		want := fmt.Sprintf("release-gate: %s for tag %s at %s\n", noMatchPrefix, "v1.2.3", tagSHA)
		msg := errb.String()
		if code != exitRetryable || msg != want || strings.HasPrefix(msg, "release-gate: pending") {
			t.Fatalf("code %d\n%s", code, msg)
		}
	})
	t.Run("red prerelease", func(t *testing.T) {
		const tag = "v1.0.0-pending"
		installGH(t, listJSON(9, "completed", "success", tag, tagSHA), "  cat <<'EOF'\n"+jobsJSON("failure")+"EOF\n")
		err := requireGreenCI(tag, tagSHA)
		if err == nil || retryable(err) || !strings.Contains(err.Error(), "not green") {
			t.Fatalf("err=%v", err)
		}
		var errb bytes.Buffer
		if code := run([]string{"-require-ci", "-tag", tag, "-sha", tagSHA}, &errb); code != 1 || !strings.Contains(errb.String(), "not green") {
			t.Fatalf("code %d\n%s", code, errb.String())
		}
	})
	t.Run("sha pending", func(t *testing.T) {
		const tag = "v1.0.0-pending"
		installGH(t, listJSON(9, "completed", "success", tag, "pending"), viewGreen())
		var errb bytes.Buffer
		if code := run([]string{"-require-ci", "-tag", tag, "-sha", "pending"}, &errb); code != 1 {
			t.Fatalf("code %d\n%s", code, errb.String())
		}
	})
	t.Run("gh list fails", func(t *testing.T) {
		installGHListFails(t)
		var errb bytes.Buffer
		code := run([]string{"-require-ci", "-tag", "v1.0.0-pending", "-sha", tagSHA}, &errb)
		msg := errb.String()
		if code != 1 || strings.HasPrefix(msg, "release-gate: pending") || strings.HasPrefix(msg, "release-gate: no matching run") {
			t.Fatalf("code %d\n%s", code, msg)
		}
	})
	t.Run("prerelease in progress", func(t *testing.T) {
		const tag = "v1.0.0-pending"
		installGH(t, listJSON(41, "in_progress", "", tag, tagSHA), "  echo 'view must not run' >&2\n  exit 9\n")
		var errb bytes.Buffer
		code := run([]string{"-require-ci", "-tag", tag, "-sha", tagSHA}, &errb)
		want := fmt.Sprintf("release-gate: %s CI run %d for tag %s\n", pendingPrefix, 41, tag)
		msg := errb.String()
		if code != exitRetryable || msg != want || !strings.HasPrefix(msg, "release-gate: pending ") || !strings.Contains(msg, tag) {
			t.Fatalf("code %d\n%s", code, msg)
		}
	})
	t.Run("vpending", func(t *testing.T) {
		installGH(t, listJSON(7, "completed", "success", "vpending", tagSHA), viewGreen())
		var errb bytes.Buffer
		code := run([]string{"-require-ci", "-tag", "vpending", "-sha", tagSHA}, &errb)
		if code != 1 || strings.Contains(errb.String(), "vpending") {
			t.Fatalf("code %d\n%s", code, errb.String())
		}
	})
	t.Run("duplicate failure then success", func(t *testing.T) {
		body := "  cat <<'EOF'\n" + viewJobs(map[string][]string{"unit": {"failure", "success"}}, "") + "\nEOF\n"
		installGH(t, listJSON(7, "completed", "success", "v1.2.3", tagSHA), body)
		var errb bytes.Buffer
		code := run([]string{"-require-ci", "-tag", "v1.2.3", "-sha", tagSHA}, &errb)
		msg := errb.String()
		if code != 1 || code == exitRetryable ||
			!strings.Contains(msg, "exactly once") || !strings.Contains(msg, "unit=failure") ||
			strings.HasPrefix(msg, "release-gate: pending") || strings.HasPrefix(msg, "release-gate: no matching run") {
			t.Fatalf("code %d\n%s", code, msg)
		}
	})
	t.Run("duplicate both success", func(t *testing.T) {
		body := "  cat <<'EOF'\n" + viewJobs(map[string][]string{"unit": {"success", "success"}}, "") + "\nEOF\n"
		installGH(t, listJSON(7, "completed", "success", "v1.2.3", tagSHA), body)
		var errb bytes.Buffer
		code := run([]string{"-require-ci", "-tag", "v1.2.3", "-sha", tagSHA}, &errb)
		msg := errb.String()
		if code != 1 || code == exitRetryable ||
			!strings.Contains(msg, "exactly once") || strings.Contains(msg, "unit=failure") ||
			strings.HasPrefix(msg, "release-gate: pending") || strings.HasPrefix(msg, "release-gate: no matching run") {
			t.Fatalf("code %d\n%s", code, msg)
		}
	})
	t.Run("missing job", func(t *testing.T) {
		body := "  cat <<'EOF'\n" + viewJobs(nil, "web") + "\nEOF\n"
		installGH(t, listJSON(7, "completed", "success", "v1.2.3", tagSHA), body)
		var errb bytes.Buffer
		code := run([]string{"-require-ci", "-tag", "v1.2.3", "-sha", tagSHA}, &errb)
		msg := errb.String()
		if code != 1 || code == exitRetryable ||
			!strings.Contains(msg, "web=missing") || !strings.Contains(msg, "not green") ||
			strings.HasPrefix(msg, "release-gate: pending") || strings.HasPrefix(msg, "release-gate: no matching run") {
			t.Fatalf("code %d\n%s", code, msg)
		}
	})
	t.Run("extra failed job ignored", func(t *testing.T) {
		body := "  cat <<'EOF'\n" + viewJobs(map[string][]string{"apidiff": {"failure"}}, "") + "\nEOF\n"
		installGH(t, listJSON(7, "completed", "success", "v1.2.3", tagSHA), body)
		var errb bytes.Buffer
		code := run([]string{"-require-ci", "-tag", "v1.2.3", "-sha", tagSHA}, &errb)
		if code != 0 {
			t.Fatalf("code %d\n%s", code, errb.String())
		}
	})
}

// listJSON is one push run. headSha is the -sha passed to release-gate.
func listJSON(id int, status, conclusion, branch, sha string) string {
	return fmt.Sprintf(`[{"databaseId":%d,"conclusion":%q,"status":%q,"headSha":%q,"event":"push","headBranch":%q}]`,
		id, conclusion, status, sha, branch)
}

// installGHListFails is a gh that prints the old retry words and exits 1.
func installGHListFails(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "gh")
	body := "#!/bin/sh\necho pending >&2\necho 'no matching run' >&2\nexit 1\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GH_TOKEN", "test")
}
