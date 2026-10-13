package app

import (
	"math"
	"sort"
	"testing"

	"github.com/zhoushoujianwork/easyeda-agent/internal/blocks"
)

// Regression for the 2026-10-13 ESP32-S3 demo: block.esp32s3_wroom1_module
// threw its attach parts 270–540 units sideways (one 100nF decoupler sat 540
// from the 3V3 pin it is declared to hug), interleaved the 3V3 and EN groups,
// and dropped two parts on the title block. Geometry is the demo's measured
// anchor (bbox [314,240]-[386,660]; 3V3/EN on the left column, IO0 top right).
func TestBlockAttachDemoStaysOnItsPinsAndOffTheTitleBlock(t *testing.T) {
	// Second case: the live readback reports half-unit edges (host top 660.5);
	// rounding the seed back toward the host rejected every attach seed.
	for _, anchor := range []layoutBBox{{MinX: 314, MinY: 240, MaxX: 386, MaxY: 660}, {MinX: 314.5, MinY: 239.5, MaxX: 385.5, MaxY: 660.5}} {
		checkBlockAttachDemo(t, anchor)
	}
}

func checkBlockAttachDemo(t *testing.T, anchor layoutBBox) {
	t.Helper()
	blk, ok, err := blocks.Get("block.esp32s3_wroom1_module")
	if err != nil || !ok {
		t.Fatal("block missing", err)
	}
	sl, err := blk.SchematicLayout()
	if err != nil {
		t.Fatal(err)
	}
	rel, ok := bslRelationsFrom(sl)
	if !ok {
		t.Fatal("no relations")
	}
	r180, r0 := 180.0, 0.0
	pins := map[string]acPin{
		"3V3": {X: 305, Y: 465, PinName: "3V3", PinNumber: "2", PinRotation: &r180},
		"EN":  {X: 305, Y: 455, PinName: "EN", PinNumber: "3", PinRotation: &r180},
		"IO0": {X: 395, Y: 650, PinName: "IO0", PinNumber: "27", PinRotation: &r0},
	}
	sheet := layoutBBox{MinX: 0, MinY: 0, MaxX: 1169, MaxY: 825}
	usable := schUsableArea(sheet)
	tb, _ := titleBlockKeepout(&sheet)
	solved, notes := bslSolveAround(blk, rel, bslBlockNets(blk), nil, "U", pins, anchor, []layoutBBox{anchor, *tb}, &usable)
	if len(solved) != 5 {
		t.Fatalf("all five attach parts must solve, got %d (notes %v)", len(solved), notes)
	}
	at := map[string]bslSolved{}
	for _, s := range solved {
		at[s.Role] = s
		if s.Source != "attach" {
			t.Errorf("%s fell back to %s", s.Role, s.Source)
		}
		box := bslPartBox(blk.Parts[s.Role].Part)
		body := layoutBBox{MinX: s.X + box.MinX, MinY: s.Y + box.MinY, MaxX: s.X + box.MaxX, MaxY: s.Y + box.MaxY}
		if !boxInside(body, usable) || boxesGapOverlap(body, *tb, 0) {
			t.Errorf("%s at (%.0f,%.0f) leaves the sheet or sits on the title block", s.Role, s.X, s.Y)
		}
		// The host-facing end of a vertical part needs its own stub + marker lane
		// (live: C_VDD over the body had its 3V3 stub rejected by the guard).
		if s.Rotation == 270 && body.MinY > anchor.MaxY && body.MinY-anchor.MaxY < bslMarkerVReach {
			t.Errorf("%s leaves only %.1f between its host-facing end and the host (need ≥ %.0f)", s.Role, body.MinY-anchor.MaxY, bslMarkerVReach)
		}
	}
	near := map[string]float64{"C_VDD": 305, "C_BULK": 305, "R_EN": 305, "C_EN": 305, "R_IO0": 395}
	for role, px := range near {
		if d := math.Abs(at[role].X - px); d > 100 {
			t.Errorf("%s is %.0f from its pin column (want ≤100)", role, d)
		}
	}
	// Each pin's parts are contiguous in the row (no other group in between).
	order := append([]bslSolved(nil), solved...)
	sort.Slice(order, func(i, j int) bool { return order[i].X < order[j].X })
	groupOf := map[string]string{"C_VDD": "3V3", "C_BULK": "3V3", "R_EN": "EN", "C_EN": "EN", "R_IO0": "IO0"}
	seen := map[string]bool{}
	prev := ""
	for _, s := range order {
		g := groupOf[s.Role]
		if g != prev && seen[g] {
			t.Errorf("group %s is split in the row: %+v", g, order)
		}
		seen[g], prev = true, g
	}
}
