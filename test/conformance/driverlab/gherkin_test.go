package driverlab

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

//go:embed features/driver.feature
var driverFeature string

var providers = map[string]func(*testing.T) fixture{
	"postgres":  openPostgres,
	"sqlite":    openSQLite,
	"sqlserver": openSQLServer,
	"mysql":     openMySQL,
}

type step struct {
	text string
	run  func(*scenario)
}

var setupStep = step{"Given isolated parent and child tables on <provider> with a separate observer connection", (*scenario).setup}

// Each scenario has a complete ordered contract. Removing an assertion from
// the feature must fail discovery just as adding an unbound step would.
var bindings = map[string][]step{
	"LAB-DRIVER-001-S01": {
		setupStep,
		{"When the writer commits a transaction inserting a parent", (*scenario).commitParent},
		{"Then the observer reads the committed parent", (*scenario).checkParent},
		{"When the writer rolls back a transaction inserting a second parent", (*scenario).rollbackParent},
		{"Then the observer still reads only the committed parent", (*scenario).checkParent},
	},
	"LAB-DRIVER-001-S02": {
		setupStep,
		{"And a parent with a unique code exists", (*scenario).seedParent},
		{"When the writer inserts another parent with the same code", (*scenario).duplicateParent},
		{"Then the driver reports the native unique constraint code", (*scenario).checkUniqueCode},
		{"And the observer reads only the original parent", (*scenario).checkParent},
	},
	"LAB-DRIVER-001-S03": {
		setupStep,
		{"And a parent and its child exist", (*scenario).seedFamily},
		{"When the writer inserts another child referencing a missing parent", (*scenario).orphanChild},
		{"Then the driver reports the native foreign key code", (*scenario).checkForeignKeyCode},
		{"And the observer reads only the original parent and child", (*scenario).checkFamily},
	},
	"LAB-DRIVER-001-S04": {
		{"Given a single-connection provider SPI fixture on <provider>", (*scenario).setupSPICancel},
		{"When the reader reads the first row and the caller cancels", (*scenario).cancelSPIReader},
		{"Then the reader reports caller cancellation and releases its connection", (*scenario).checkSPICancellation},
		{"And a fresh reader reads both rows and releases its connection", (*scenario).checkSPIRecovery},
	},
}

type example struct {
	scenarioID string
	provider   string
	steps      []step
}

func parseDriverFeature(source string) ([]example, error) {
	var lines []string
	for line := range strings.SplitSeq(source, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	next := func() string {
		if len(lines) == 0 {
			return ""
		}
		line := lines[0]
		lines = lines[1:]
		return line
	}
	header := func(keyword string) bool {
		title, ok := strings.CutPrefix(next(), keyword+": ")
		return ok && strings.TrimSpace(title) != ""
	}
	if next() != "@LAB-DRIVER-001" || !header("Feature") {
		return nil, fmt.Errorf("expected the driver feature tag and a named Feature")
	}

	var examples []example
	seen := make(map[string]bool)
	for len(lines) > 0 {
		id, tagged := strings.CutPrefix(next(), "@")
		steps, known := bindings[id]
		if !tagged || !known || seen[id] || !header("Scenario Outline") {
			return nil, fmt.Errorf("expected a uniquely tagged, bound Scenario Outline")
		}
		seen[id] = true
		for _, step := range steps {
			if next() != step.text {
				return nil, fmt.Errorf("%s: expected %q", id, step.text)
			}
		}
		if next() != "Examples:" || tableCell(next()) != "provider" {
			return nil, fmt.Errorf("%s: expected Examples with a provider column", id)
		}
		rows := make(map[string]bool)
		for len(lines) > 0 && strings.HasPrefix(lines[0], "|") {
			provider := tableCell(next())
			if providers[provider] == nil || rows[provider] {
				return nil, fmt.Errorf("%s: unexpected or duplicate provider row", id)
			}
			rows[provider] = true
			examples = append(examples, example{id, provider, steps})
		}
		if len(rows) != len(providers) {
			return nil, fmt.Errorf("%s: incomplete provider matrix", id)
		}
	}
	if len(seen) != len(bindings) {
		return nil, fmt.Errorf("incomplete driver scenario matrix")
	}
	return examples, nil
}

func tableCell(line string) string {
	parts := strings.Split(line, "|")
	if len(parts) != 3 || parts[0] != "" || parts[2] != "" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func TestDriverFeature(t *testing.T) {
	examples, err := parseDriverFeature(driverFeature)
	if err != nil || len(examples) != 16 {
		t.Fatalf("expected sixteen bound examples, got %d: %v", len(examples), err)
	}
	for _, test := range []struct{ name, before, after string }{
		{"unknown step", "When the writer commits", "When the reader commits"},
		{"missing assertion", "Then the observer reads the committed parent", ""},
		{"wrong keyword", "And a parent with a unique code exists", "But a parent with a unique code exists"},
		{"unknown provider", "| mysql     |", "| remote |"},
		{"missing provider", "| sqlite    |", ""},
		{"duplicate provider", "| sqlite    |", "| postgres |"},
		{"extra column", "| provider  |", "| provider | other |"},
		{"unclosed row", "| mysql     |", "| mysql"},
		{"duplicate scenario", "@LAB-DRIVER-001-S03", "@LAB-DRIVER-001-S02"},
		{"unknown scenario", "@LAB-DRIVER-001-S03", "@LAB-DRIVER-001-S99"},
		{"missing cancellation assertion", "Then the reader reports caller cancellation and releases its connection", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := strings.Replace(driverFeature, test.before, test.after, 1)
			if _, err := parseDriverFeature(source); err == nil {
				t.Fatal("invalid driver feature passed discovery")
			}
		})
	}
	withoutLast, _, _ := strings.Cut(driverFeature, "@LAB-DRIVER-001-S04")
	if _, err := parseDriverFeature(withoutLast); err == nil {
		t.Fatal("missing scenario passed discovery")
	}
}

type scenarioResult struct {
	Provider string            `json:"provider"`
	Scenario string            `json:"scenario"`
	Status   string            `json:"status"`
	Facts    map[string]string `json:"facts,omitempty"`
}

func TestDriverGherkin(t *testing.T) {
	examples, err := parseDriverFeature(driverFeature)
	if err != nil {
		t.Fatal(err)
	}
	provider, selected := os.LookupEnv("GOEF_DRIVERLAB_PROVIDER")
	if !selected {
		t.Skip("set GOEF_DRIVERLAB_PROVIDER using the isolated lab runner")
	}
	if providers[provider] == nil {
		t.Fatal("unknown driver lab provider")
	}
	for _, row := range examples {
		if row.provider != provider {
			continue
		}
		var result *scenarioResult
		passed := t.Run(row.provider+"/"+row.scenarioID, func(t *testing.T) {
			s := scenario{t: t, provider: row.provider}
			for _, step := range row.steps {
				step.run(&s)
			}
			result = &scenarioResult{Provider: row.provider, Scenario: row.scenarioID, Status: "PASS", Facts: s.facts}
		})
		// t.Run returns after fixture cleanup. A filtered or skipped example
		// has no completed result and cannot produce a passing marker.
		if passed && result != nil {
			report, err := json.Marshal(result)
			if err != nil {
				t.Fatal("could not encode driver lab result")
			}
			fmt.Printf("GOEF_DRIVERLAB_RESULT=%s\n", report)
		}
	}
}
