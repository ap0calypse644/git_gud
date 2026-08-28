package detector

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/timeline"
)

func TestBadFightJoinEvidenceSerializesEmptyAlliedDeathsAsArray(t *testing.T) {
	got := assessBadFightJoin(timeline.TargetFightContext{})
	b, err := json.Marshal(got.Evidence)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"allied_deaths_before_involvement":[]`) {
		t.Fatalf("expected explicit empty allied-deaths array, got %s", b)
	}
}
