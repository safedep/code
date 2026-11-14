//go:build projectgraph

package projectgraph

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// govulncheckVersion is the version of govulncheck that the method measure
// compares with. Install it with
// go install golang.org/x/vuln/cmd/govulncheck@v1.9.0.
const govulncheckVersion = "v1.9.0"

// vulnCheck runs govulncheck on each Go project of the method measure and
// compares its findings with the answers of reach. A zero vulnCheck
// compares nothing.
type vulnCheck struct {
	bin, db, gomod string
	// listed holds the import paths of each advisory that list symbols.
	// govulncheck reports any called symbol of a module or a package that
	// an advisory names with no symbol list, and the measure has no target
	// for such an advisory.
	listed    map[string]map[string]bool
	agreement vulnAgreement
}

// newVulnCheck finds govulncheck on the PATH and writes a vulnerability
// database of the Go advisories of the measure, so govulncheck checks
// exactly these advisories and needs no network.
func newVulnCheck(t *testing.T, corpus string, advisories map[string][]advisory, measureGo bool) *vulnCheck {
	if !measureGo {
		return &vulnCheck{}
	}
	bin, err := exec.LookPath("govulncheck")
	if err != nil {
		t.Logf("govulncheck is not on the PATH, so the measure does not compare with it: install govulncheck@%s", govulncheckVersion)
		return &vulnCheck{}
	}
	db := t.TempDir()
	writeVulnDB(t, db, advisories)
	listed := map[string]map[string]bool{}
	for _, list := range advisories {
		for _, a := range list {
			listed[a.ID] = map[string]bool{}
			for _, imp := range a.Imports {
				listed[a.ID][imp.Path] = len(imp.Symbols) > 0
			}
		}
	}
	return &vulnCheck{bin: bin, db: db, gomod: filepath.Join(corpus, "gomod"), listed: listed}
}

// compare runs govulncheck on the project in dir and adds the answers of
// reach for the project to the agreement.
func (v *vulnCheck) compare(t *testing.T, project, dir string, answers []methodAnswer) {
	if v.bin == "" {
		return
	}
	v.agreement.add(project, answers, v.reached(t, dir))
}

func (v *vulnCheck) rows() []string {
	if v.bin == "" {
		return nil
	}
	return v.agreement.rows()
}

// reached returns the keys of the vulnerable symbols that govulncheck
// finds a call stack to in the project in dir. The module cache of the
// corpus holds the build list of the project, so the go command runs with
// no proxy. It may add go.sum lines, so it runs on a copy of the project.
func (v *vulnCheck) reached(t *testing.T, dir string) map[string]bool {
	work := t.TempDir()
	require.NoError(t, os.CopyFS(work, os.DirFS(dir)))
	cmd := exec.Command(v.bin, "-mode", "source", "-json", "-db", "file://"+v.db, "./...")
	cmd.Dir = work
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOFLAGS=-mod=mod", "GOMODCACHE="+v.gomod, "GOTOOLCHAIN=local", "GOWORK=off")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	require.NoError(t, err, stderr.String())
	return v.reachedIn(t, out)
}

// reachedIn returns the keys of the vulnerable symbols that the findings
// of a govulncheck -json output reach with a call stack, for the advisories
// that list symbols. It fails when the output comes from another version
// of govulncheck, because the call graph of another version can reach
// other symbols.
func (v *vulnCheck) reachedIn(t *testing.T, out []byte) map[string]bool {
	reached := map[string]bool{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var m struct {
			vulnMessage
			Config *struct {
				ScannerVersion string `json:"scanner_version"`
			} `json:"config"`
		}
		err := dec.Decode(&m)
		if errors.Is(err, io.EOF) {
			return reached
		}
		require.NoError(t, err)
		if m.Config != nil {
			require.Equal(t, govulncheckVersion, m.Config.ScannerVersion, "install govulncheck@%s", govulncheckVersion)
		}
		if f := m.Finding; f != nil && len(f.Trace) > 0 && f.Trace[0].Function != "" && v.listed[f.OSV][f.Trace[0].Package] {
			reached[frameKey(f.Trace[0])] = true
		}
	}
}

// writeVulnDB writes the Go advisories of the measure as a vulnerability
// database in the layout of vuln.go.dev, which govulncheck -db file://
// reads: index/db.json, index/modules.json and ID/<id>.json.
func writeVulnDB(t *testing.T, dir string, advisories map[string][]advisory) {
	modified := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	type vuln struct {
		ID       string    `json:"id"`
		Modified time.Time `json:"modified"`
	}
	type module struct {
		Path  string `json:"path"`
		Vulns []vuln `json:"vulns"`
	}
	var index []module
	entries := map[string][]any{}
	for _, path := range slices.Sorted(maps.Keys(advisories)) {
		m := module{Path: path}
		for _, a := range advisories[path] {
			m.Vulns = append(m.Vulns, vuln{ID: a.ID, Modified: modified})
			entries[a.ID] = append(entries[a.ID], map[string]any{
				"package":            map[string]string{"name": path, "ecosystem": "Go"},
				"ranges":             []any{map[string]any{"type": "SEMVER", "events": a.Ranges}},
				"ecosystem_specific": map[string]any{"imports": a.Imports},
			})
		}
		index = append(index, m)
	}
	writeJSON(t, filepath.Join(dir, "index", "db.json"), map[string]time.Time{"modified": modified})
	writeJSON(t, filepath.Join(dir, "index", "modules.json"), index)
	for id, affected := range entries {
		writeJSON(t, filepath.Join(dir, "ID", id+".json"), map[string]any{
			"schema_version": "1.3.1",
			"id":             id,
			"modified":       modified,
			"details":        "",
			"affected":       affected,
		})
	}
}

func writeJSON(t *testing.T, file string, v any) {
	data, err := json.Marshal(v)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o755))
	require.NoError(t, os.WriteFile(file, data, 0o644))
}

// The classes of a reach answer for the comparison with govulncheck.
const (
	answerFound     = "found"
	answerMissing   = "not-found"
	answerUncertain = "uncertain"
)

// answerClass returns the class of an outcome of the measure. found/low
// is found, and no-such-target says that no path exists, as not-found does.
func answerClass(outcome string) string {
	switch status, _, _ := strings.Cut(outcome, "/"); status {
	case "found":
		return answerFound
	case "not-found", "no-such-target":
		return answerMissing
	}
	return answerUncertain
}

// vulnAgreement counts the answers of reach for the targets that
// govulncheck reaches and for the targets that it does not reach, by the
// class of the answer. A missing answer for a target that govulncheck
// reaches is a false negative. A found answer for a target that
// govulncheck does not reach is a row to review, because reach also starts
// from the library surface of the project. A symbol that govulncheck
// reaches and that is no target of the project shows a key that the two
// tools write in two ways, which would hide a false negative.
type vulnAgreement struct {
	reached, quiet                    map[string]int
	falseNegatives, review, unmatched []string
}

func (a *vulnAgreement) add(project string, answers []methodAnswer, reached map[string]bool) {
	if a.reached == nil {
		a.reached, a.quiet = map[string]int{}, map[string]int{}
	}
	targets := map[string]bool{}
	for _, ans := range answers {
		targets[ans.key] = true
		class := answerClass(ans.outcome)
		switch {
		case !reached[ans.key]:
			a.quiet[class]++
			if class == answerFound {
				a.review = append(a.review, ans.detail)
			}
		case class == answerMissing:
			a.reached[class]++
			a.falseNegatives = append(a.falseNegatives, ans.detail)
		default:
			a.reached[class]++
		}
	}
	for _, key := range slices.Sorted(maps.Keys(reached)) {
		if !targets[key] {
			a.unmatched = append(a.unmatched, "project="+project+" symbol="+key)
		}
	}
}

// rows writes the counts of each direction, the share of each direction
// on which the two tools agree, and one row per false negative, per
// answer to review and per unmatched symbol.
func (a vulnAgreement) rows() []string {
	total := func(m map[string]int) int {
		n := 0
		for _, c := range m {
			n += c
		}
		return n
	}
	reached, quiet := total(a.reached), total(a.quiet)
	rows := []string{
		"set=govulncheck-agreement language=go version=" + govulncheckVersion,
		"  govulncheck-reached=" + strconv.Itoa(reached) + " found=" + strconv.Itoa(a.reached[answerFound]) + " uncertain=" + strconv.Itoa(a.reached[answerUncertain]) + " false-negative=" + strconv.Itoa(len(a.falseNegatives)),
		"  govulncheck-not-reached=" + strconv.Itoa(quiet) + " not-found=" + strconv.Itoa(a.quiet[answerMissing]) + " uncertain=" + strconv.Itoa(a.quiet[answerUncertain]) + " found=" + strconv.Itoa(a.quiet[answerFound]),
		"  agree-reached=" + ratio{hit: a.reached[answerFound], all: reached}.String() + " agree-not-reached=" + ratio{hit: a.quiet[answerMissing], all: quiet}.String(),
		"  unmatched=" + strconv.Itoa(len(a.unmatched)),
	}
	for _, group := range []struct {
		name  string
		lines []string
	}{{"false-negative", a.falseNegatives}, {"review", a.review}, {"unmatched", a.unmatched}} {
		for _, line := range group.lines {
			rows = append(rows, "  "+group.name+" "+line)
		}
	}
	return rows
}

func TestVulnAgreement(t *testing.T) {
	answer := func(key, outcome string) methodAnswer {
		return methodAnswer{key: key, outcome: outcome, detail: "target=" + key + " outcome=" + outcome}
	}
	answers := []methodAnswer{
		answer("x//Missed", "not-found"),
		answer("x//Absent", "no-such-target"),
		answer("x//Called", "found/low"),
		answer("x//Unsure", "uncertain/ambiguous"),
		answer("x//Extra", "found"),
		answer("x//Quiet", "not-found"),
	}
	reached := map[string]bool{"x//Missed": true, "x//Absent": true, "x//Called": true, "x//Unsure": true, "x//Other": true}

	var a vulnAgreement
	a.add("p", answers, reached)

	assert.Equal(t, []string{"target=x//Missed outcome=not-found", "target=x//Absent outcome=no-such-target"}, a.falseNegatives)
	assert.Equal(t, []string{"target=x//Extra outcome=found"}, a.review)
	assert.Equal(t, []string{"project=p symbol=x//Other"}, a.unmatched)
	assert.Equal(t, map[string]int{answerMissing: 2, answerFound: 1, answerUncertain: 1}, a.reached)
	assert.Equal(t, map[string]int{answerFound: 1, answerMissing: 1}, a.quiet)
	assert.Contains(t, a.rows(), "  govulncheck-reached=4 found=1 uncertain=1 false-negative=2")
}
