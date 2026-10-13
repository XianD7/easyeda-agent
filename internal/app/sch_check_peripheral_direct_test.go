package app

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Fixtures are trimmed live components.list reads of the 2026-10-13 ESP32-S3
// demo page P1 (ceshi): labels-only.json is the block-apply/autoconnect result
// (every decoupler/pull-up a lone part tied to U1 by net labels, all gates green),
// composed.json the same parts after `sch compose --preserve-instances`.
func TestPeripheralDirectFindingCatchesLabelOnlyModules(t *testing.T) {
	load := func(name string) map[string]any {
		raw, err := os.ReadFile("testdata/peripheral-direct/" + name)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	groups := []*schGroup{
		{ID: "g1", Name: "MCU", Members: []string{"U1", "C3", "C1", "R1", "C2", "R2"}},
		{ID: "g2", Name: "LED", Members: []string{"LED1", "R3"}},
		{ID: "g3", Name: "PWR", Members: []string{"J1"}},
	}
	got := peripheralDirectFindingsFor(load("labels-only.json"), groups)
	if len(got) == 0 || got[0].Type != "peripheral-not-direct" || !strings.Contains(got[0].Message, "MCU") || got[0].Designator != "U1" {
		t.Fatalf("label-only MCU module must be flagged with U1 as core, got %+v", got)
	}
	if got := peripheralDirectFindingsFor(load("composed.json"), groups); len(got) != 0 {
		t.Fatalf("composed page passes compose's own ruler, got %+v", got)
	}
	// block-apply subgroups of one instance merge: U1 is in a different subgroup
	// than its decouplers, yet they form one module with U1 as core.
	sub := []*schGroup{
		{ID: "g1", BlockID: "esp32", Instance: "i1", Members: []string{"C1", "C3"}},
		{ID: "g2", BlockID: "esp32", Instance: "i1", Members: []string{"U1"}},
	}
	if got := peripheralDirectFindingsFor(load("composed.json"), sub); len(got) != 0 {
		t.Fatalf("merged subgroups judge as one module, got %+v", got)
	}
}
