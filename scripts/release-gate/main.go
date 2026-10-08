// Command release-gate validates release notes headings and required CI on a SHA.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var requiredHeadings = []string{
	"Highlights",
	"Added",
	"Residual",
	"Deployment and operations",
	"CI and release evidence",
}

var requiredCIJobs = []string{
	"format", "lint", "unit", "race", "fuzz-smoke", "documentation",
	"config-compat", "changelog", "generated-file", "parity", "security-scan",
	"container-test", "web",
}

func main() {
	notesOnly := flag.Bool("notes-only", false, "validate notes headings only")
	notes := flag.String("notes", "", "path to docs/releases/vX.Y.Z.md")
	requireCI := flag.Bool("require-ci", false, "require green CI on GITHUB_SHA or HEAD")
	flag.Parse()
	if *notesOnly {
		if *notes == "" {
			fatal(fmt.Errorf("-notes-only requires -notes"))
		}
		if err := validateNotes(*notes); err != nil {
			fatal(err)
		}
		return
	}
	if *requireCI {
		if err := requireGreenCI(); err != nil {
			fatal(err)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "usage: release-gate -notes-only -notes PATH | -require-ci\n")
	os.Exit(2)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "release-gate: %v\n", err)
	os.Exit(1)
}

func validateNotes(path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(body)
	for _, h := range requiredHeadings {
		if !strings.Contains(text, "## "+h) && !strings.Contains(text, "# "+h) {
			return fmt.Errorf("notes %s missing heading %q", path, h)
		}
	}
	for _, bad := range []string{"TODO", "TBD", "FIXME"} {
		if strings.Contains(text, bad) {
			return fmt.Errorf("notes %s contains %s", path, bad)
		}
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "ships netconf over tls") || strings.Contains(lower, "call-home is supported") {
		return fmt.Errorf("notes %s must not claim 1.0 TLS/call-home", path)
	}
	return nil
}

func releaseTag() (string, error) {
	ref := os.Getenv("GITHUB_REF")
	var tag string
	if strings.HasPrefix(ref, "refs/tags/") {
		tag = strings.TrimPrefix(ref, "refs/tags/")
	} else {
		tag = strings.TrimPrefix(os.Getenv("GITHUB_REF_NAME"), "refs/tags/")
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", fmt.Errorf("release tag is empty")
	}
	return tag, nil
}

func requireGreenCI() error {
	sha := strings.TrimSpace(os.Getenv("GITHUB_SHA"))
	if sha == "" {
		out, err := exec.Command("git", "rev-parse", "HEAD").Output()
		if err != nil {
			return fmt.Errorf("rev-parse HEAD: %w", err)
		}
		sha = strings.TrimSpace(string(out))
	}
	tag, err := releaseTag()
	if err != nil {
		return err
	}
	cmd := exec.Command("gh", "run", "list",
		"--workflow=ci.yml",
		"--commit="+sha,
		"--limit=200",
		"--json", "databaseId,status,headSha,headBranch,event")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("gh run list: %w", err)
	}
	var runs []struct {
		DatabaseID int    `json:"databaseId"`
		Status     string `json:"status"`
		HeadSHA    string `json:"headSha"`
		HeadBranch string `json:"headBranch"`
		Event      string `json:"event"`
	}
	if err := json.Unmarshal(out, &runs); err != nil {
		return fmt.Errorf("parse gh run list: %w", err)
	}
	bestIdx := -1
	for i, r := range runs {
		if r.Event != "push" || r.HeadSHA != sha || r.HeadBranch != tag {
			continue
		}
		if bestIdx < 0 || r.DatabaseID > runs[bestIdx].DatabaseID {
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return fmt.Errorf("no matching run for tag %s at %s", tag, sha)
	}
	best := runs[bestIdx]
	if best.Status != "completed" {
		return fmt.Errorf("pending CI run %d for tag %s", best.DatabaseID, tag)
	}
	view := exec.Command("gh", "run", "view", fmt.Sprintf("%d", best.DatabaseID), "--json", "jobs")
	jobJSON, err := view.Output()
	if err != nil {
		return fmt.Errorf("gh run view: %w", err)
	}
	var payload struct {
		Jobs []struct {
			Name       string `json:"name"`
			Conclusion string `json:"conclusion"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(jobJSON, &payload); err != nil {
		return fmt.Errorf("parse jobs: %w", err)
	}
	got := map[string]string{}
	for _, j := range payload.Jobs {
		got[j.Name] = j.Conclusion
	}
	var missing []string
	for _, name := range requiredCIJobs {
		if got[name] != "success" {
			missing = append(missing, fmt.Sprintf("%s=%s", name, got[name]))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required CI jobs not green: %s", strings.Join(missing, ", "))
	}
	return nil
}
