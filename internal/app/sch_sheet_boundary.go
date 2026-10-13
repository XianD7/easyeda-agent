package app

import (
	"fmt"
	"math"
)

// sheetBoundaryFindings is sch check's sheet-boundary ruler over EVERY drawn
// primitive — parts, markers AND wires. Live 2026-10-13: wires crossed the title
// block and labels left the frame, yet titleblock-overlap (parts/markers only),
// layout-lint (part bodies only) and the electrical rules all stayed at 0; only
// `sch clusters --strict` saw the off-sheet part, through a coarse group bbox.
//
// It reuses the page geometry the other rulers use — sheetBBoxOf, schUsableArea
// and the title-block keep-out — so "inside the sheet" means one thing.
//
//	out-of-sheet        error  part/marker bbox or wire segment leaves the usable area
//	titleblock-overlap  warn   a wire segment enters the title-block keep-out
//	                           (info when the keep-out is an estimate, as for parts)
//
// No sheet (paperless) → no findings: there is no boundary to judge.
func sheetBoundaryFindings(comps []layoutComp, wires []schGroupWire, sheet, titleBlock *layoutBBox, tbSource string, eps float64) []checkFinding {
	if sheet == nil {
		return nil
	}
	usable := schUsableArea(*sheet)
	inside := func(b layoutBBox) bool {
		return b.MinX >= usable.MinX-eps && b.MaxX <= usable.MaxX+eps && b.MinY >= usable.MinY-eps && b.MaxY <= usable.MaxY+eps
	}
	var out []checkFinding
	for _, c := range comps {
		if c.BBox == nil || (c.ComponentType != schLayoutPartType && c.ComponentType != "" && !isSchMarker(c.ComponentType)) {
			continue // the sheet itself and unmeasured primitives are not judged
		}
		if inside(*c.BBox) {
			continue
		}
		b := *c.BBox
		out = append(out, checkFinding{
			Type: "out-of-sheet", Level: "error", PrimitiveId: c.ID, ComponentType: c.ComponentType,
			Designator: c.Designator, MarkerNet: c.Net, BBox: c.BBox,
			Message: fmt.Sprintf("%s(%s) 超出图纸可用区（%.0f..%.0f × %.0f..%.0f，可用区 %.0f..%.0f × %.0f..%.0f）— 打印不出来，移回图内",
				markerLabel(c), c.ComponentType, b.MinX, b.MaxX, b.MinY, b.MaxY, usable.MinX, usable.MaxX, usable.MinY, usable.MaxY),
		})
	}
	var inner *layoutBBox
	if titleBlock != nil {
		// Only positive intrusion counts; a wire running along the keep-out edge does not.
		in := layoutBBox{MinX: titleBlock.MinX + eps, MinY: titleBlock.MinY + eps, MaxX: titleBlock.MaxX - eps, MaxY: titleBlock.MaxY - eps}
		if in.MaxX > in.MinX && in.MaxY > in.MinY {
			inner = &in
		}
	}
	confirmed := tbSource == sheetSourceKnownTemplate
	for _, w := range wires {
		offSheet, inTitle := false, false
		for _, s := range schDesignatorWireSegments(w) {
			seg := layoutBBox{MinX: math.Min(s[0], s[2]), MaxX: math.Max(s[0], s[2]), MinY: math.Min(s[1], s[3]), MaxY: math.Max(s[1], s[3])}
			if !offSheet && !inside(seg) {
				offSheet = true
				out = append(out, checkFinding{
					Type: "out-of-sheet", Level: "error", WirePrimitiveId: w.ID,
					At:      &checkPoint{X: round2(s[0]), Y: round2(s[1])},
					Message: fmt.Sprintf("导线 %s 的线段 (%.0f,%.0f)→(%.0f,%.0f) 超出图纸可用区 — 连同它挂的标签移回图内", w.ID, s[0], s[1], s[2], s[3]),
				})
			}
			if !inTitle && inner != nil && schSegmentCrossesBox(s[0], s[1], s[2], s[3], *inner) {
				inTitle = true
				level, msg := "warn", fmt.Sprintf("导线 %s 穿过标题栏 keep-out（线段 (%.0f,%.0f)→(%.0f,%.0f)）— 改走线或挪开所连器件", w.ID, s[0], s[1], s[2], s[3])
				if !confirmed {
					level = "info"
					msg = fmt.Sprintf("导线 %s 命中基于估计的图签区（source=%s）— keep-out 非 A4 标定，仅供参考", w.ID, tbSource)
				}
				out = append(out, checkFinding{
					Type: "titleblock-overlap", Level: level, WirePrimitiveId: w.ID, Keepout: titleBlock,
					At: &checkPoint{X: round2(s[0]), Y: round2(s[1])}, Message: msg,
				})
			}
		}
	}
	return out
}
