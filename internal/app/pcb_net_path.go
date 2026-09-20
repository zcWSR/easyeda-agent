package app

// pcb net-path — read-only, pad-to-pad routed-copper evidence.
//
// The command deliberately reuses the existing list actions instead of adding a
// connector handler: components.list(includePads), line.list (lines + arcs), and
// via.list.  It builds an undirected physical-copper graph in Go and finds one
// deterministic path between each requested pair of ordered waypoints.
//
// Scope is intentionally narrower than native DRC connectivity. Copper pours,
// fills, and PLANE layers are not inferred; a miss means "not proven by listed
// routed copper", not necessarily "the finished board is electrically open".

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const netPathGeomEps = 0.001 // mil; absorbs JSON/host floating-point round-off only

type pcbNetPathOptions struct {
	From    string
	To      string
	Through []string
	Net     string
	Layer   *int
}

type pcbNetPathStep struct {
	Kind        string           `json:"kind"`
	PrimitiveID string           `json:"primitiveId,omitempty"`
	Ref         string           `json:"ref,omitempty"`
	Layer       int              `json:"layer,omitempty"`
	WidthMil    float64          `json:"widthMil,omitempty"`
	At          *pcbNetPathPoint `json:"at,omitempty"`
	Start       *pcbNetPathPoint `json:"start,omitempty"`
	End         *pcbNetPathPoint `json:"end,omitempty"`
	ArcAngle    float64          `json:"arcAngle,omitempty"`
}

type pcbNetPathPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type pcbNetPathLeg struct {
	From       string           `json:"from"`
	To         string           `json:"to"`
	Connected  bool             `json:"connected"`
	Reason     string           `json:"reason,omitempty"`
	Layers     []int            `json:"layers,omitempty"`
	WidthsMil  []float64        `json:"widthsMil,omitempty"`
	MinWidth   *float64         `json:"minWidthMil,omitempty"`
	MaxWidth   *float64         `json:"maxWidthMil,omitempty"`
	ViaCount   int              `json:"viaCount"`
	Primitives int              `json:"primitiveCount"`
	Path       []pcbNetPathStep `json:"path,omitempty"`
}

type pcbNetPathReport struct {
	Connected      bool             `json:"connected"`
	Reason         string           `json:"reason,omitempty"`
	Net            string           `json:"net"`
	From           string           `json:"from"`
	To             string           `json:"to"`
	Through        []string         `json:"through,omitempty"`
	WaypointOrder  []string         `json:"waypointOrder"`
	RequestedLayer *int             `json:"requestedLayer,omitempty"`
	Layers         []int            `json:"layers,omitempty"`
	LayerSequence  []int            `json:"layerSequence,omitempty"`
	WidthsMil      []float64        `json:"widthsMil,omitempty"`
	MinWidth       *float64         `json:"minWidthMil,omitempty"`
	MaxWidth       *float64         `json:"maxWidthMil,omitempty"`
	ViaCount       int              `json:"viaCount"`
	Primitives     int              `json:"primitiveCount"`
	Path           []pcbNetPathStep `json:"path,omitempty"`
	Legs           []pcbNetPathLeg  `json:"legs"`
	Scope          string           `json:"scope"`
	ExcludedCopper []string         `json:"excludedCopper"`
	Limitations    []string         `json:"limitations"`
}

type pcbNetPathNode struct {
	kind string
	id   string
	ref  string
	net  string

	layer          int
	width          float64
	x, y           float64
	x1, y1, x2, y2 float64
	w, h, dia      float64
	rotation       float64
	shape          string
	shapeRound     float64
	shapeSides     int
	arcAngle       float64
}

func newPcbNetPathCmd(cfg *appConfig, window *string, stdout, stderr io.Writer) *cobra.Command {
	var opts pcbNetPathOptions
	var asJSON bool
	var layer int
	c := &cobra.Command{
		Use:   "net-path",
		Short: "Prove a pad-to-pad path through listed tracks/arcs/vias (read-only; pours excluded)",
		Long: `Build a physical-copper graph from pcb.components.list --include-pads,
pcb.line.list, and pcb.via.list, then prove one continuous path between two pads.

Pad references use DESIGNATOR.PAD, for example U2.3 or CARD1.4. Repeat --through
(or pass a comma-separated list) to require one non-repeating copper path to visit
the pads in order, such as C3.1 -> C4.1 -> U2.3. Pairwise reachability with branch
backtracking does not pass this ordered proof.
The report includes the actual primitive path, copper layers, widths, and unique via
count. The optional --net is an assertion; it must match every requested pad.
--layer constrains graph construction itself to that copper layer. Physical vias and
all tracks/arcs on other layers are excluded, so --layer 1 proves a TOP-only path
rather than finding an arbitrary path and checking its layers afterwards.

Scope: routed line/arc/via copper and placed pad copper only. Copper pours, filled
regions, and PLANE connectivity are deliberately not inferred, so a failed result
means "not proven by listed routed copper". Run native pcb drc separately for the
board-wide connection verdict. Arc bodies are traversed, but junctions to an arc are
recognized only at the arc endpoints exposed by pcb.line.list. Unsupported pad
geometry, unavailable arc readback, and overlapping primitives that make an ordered
topology ambiguous return unknown/error instead of PASS.`,
		Args: cobra.NoArgs,
		Example: `  easyeda pcb net-path --from C3.1 --through C4.1 --to U2.3 --layer 1
  easyeda pcb net-path --from U5.6 --to CN1.1 --net CANL --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("layer") {
				if !netPathCopperLayer(layer) {
					return fmt.Errorf("--layer must be a copper layer id (TOP=1, BOTTOM=2, INNER=15..44)")
				}
				opts.Layer = &layer
			}
			return runPcbNetPath(cfg, *window, opts, asJSON, stdout, stderr)
		},
	}
	c.Flags().StringVar(&opts.From, "from", "", "start pad as DESIGNATOR.PAD (required)")
	c.Flags().StringVar(&opts.To, "to", "", "destination pad as DESIGNATOR.PAD (required)")
	c.Flags().StringSliceVar(&opts.Through, "through", nil, "ordered intermediate pad(s); repeat or comma-separate")
	c.Flags().StringVar(&opts.Net, "net", "", "assert the expected net name")
	c.Flags().IntVar(&layer, "layer", 0, "constrain proof to one copper layer (for example 1=TOP); excludes physical vias")
	c.Flags().BoolVar(&asJSON, "json", false, "emit a machine-readable report")
	_ = c.MarkFlagRequired("from")
	_ = c.MarkFlagRequired("to")
	return c
}

func runPcbNetPath(cfg *appConfig, window string, opts pcbNetPathOptions, asJSON bool, stdout, stderr io.Writer) error {
	compRes, err := requestAction(cfg, "pcb.components.list", window, map[string]any{"includePads": true})
	if err != nil {
		return fmt.Errorf("pcb net-path: read pads: %w", err)
	}
	pads, err := parseNetPathPads(compRes.Result)
	if err != nil {
		return fmt.Errorf("pcb net-path: read pads: %w", err)
	}
	waypoints, net, err := resolveNetPathWaypoints(pads, opts)
	if err != nil {
		return fmt.Errorf("pcb net-path: %w", err)
	}

	linePayload := map[string]any{"net": net}
	if opts.Layer != nil {
		linePayload["layer"] = *opts.Layer
	}
	lineRes, err := requestAction(cfg, "pcb.line.list", window, linePayload)
	if err != nil {
		return fmt.Errorf("pcb net-path: read tracks/arcs for net %q: %w", net, err)
	}
	tracks, arcs, err := parseNetPathLines(lineRes.Result)
	if err != nil {
		return fmt.Errorf("pcb net-path: read tracks/arcs for net %q: %w", net, err)
	}
	var vias []pcbViaP
	if opts.Layer == nil {
		viaRes, err := requestAction(cfg, "pcb.via.list", window, map[string]any{"net": net})
		if err != nil {
			return fmt.Errorf("pcb net-path: read vias for net %q: %w", net, err)
		}
		vias, err = parseNetPathVias(viaRes.Result)
		if err != nil {
			return fmt.Errorf("pcb net-path: read vias for net %q: %w", net, err)
		}
	}

	rep, err := analyzePcbNetPath(pads, tracks, arcs, vias, opts)
	if err != nil {
		return fmt.Errorf("pcb net-path: %w", err)
	}
	// The live pre-resolution and pure analyzer must agree; this catches a future
	// parser drift before it can print evidence for the wrong pad/net.
	if rep.Net != net || !equalStrings(rep.WaypointOrder, waypoints) {
		return fmt.Errorf("pcb net-path: internal waypoint resolution changed during analysis")
	}

	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return err
		}
	} else {
		renderPcbNetPath(rep, stdout)
	}
	if !rep.Connected {
		return fmt.Errorf("pcb net-path: no continuous listed-copper path for %s", strings.Join(rep.WaypointOrder, " -> "))
	}
	return nil
}

func parseNetPathPads(result map[string]any) ([]pcbPadP, error) {
	components, err := netPathRequiredSlice(result, "components")
	if err != nil {
		return nil, err
	}
	var pads []pcbPadP
	for ci, rc := range components {
		cm, ok := rc.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("components[%d] is not an object", ci)
		}
		designator := strings.TrimSpace(asString(cm["designator"]))
		componentLabel := designator
		if componentLabel == "" {
			componentLabel = fmt.Sprintf("components[%d]", ci)
		}
		componentPads, err := netPathRequiredSlice(cm, "pads")
		if err != nil {
			return nil, fmt.Errorf("component %s: %w", componentLabel, err)
		}
		for pi, rp := range componentPads {
			pm, ok := rp.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("component %s pads[%d] is not an object", componentLabel, pi)
			}
			prefix := fmt.Sprintf("component %s pads[%d]", componentLabel, pi)
			id := strings.TrimSpace(asString(pm["primitiveId"]))
			if id == "" {
				return nil, fmt.Errorf("%s.primitiveId is missing", prefix)
			}
			number := strings.TrimSpace(asString(pm["padNumber"]))
			layer, err := netPathNumber(pm, "layer", prefix)
			if err != nil || layer != math.Trunc(layer) || (int(layer) != 1 && int(layer) != 2 && int(layer) != pcbLayerMulti) {
				return nil, fmt.Errorf("%s.layer must be TOP(1), BOTTOM(2), or MULTI(12)", prefix)
			}
			x, err := netPathNumber(pm, "x", prefix)
			if err != nil {
				return nil, err
			}
			y, err := netPathNumber(pm, "y", prefix)
			if err != nil {
				return nil, err
			}
			rotation, err := netPathNumber(pm, "rotation", prefix)
			if err != nil {
				return nil, err
			}
			p := pcbPadP{ID: id, Designator: designator, Number: number, Net: asString(pm["net"]), Layer: int(layer), X: x, Y: y, Rotation: rotation}
			if w, ok := netPathOptionalFinite(pm["width"]); ok && w > 0 {
				p.W = w
			}
			if h, ok := netPathOptionalFinite(pm["height"]); ok && h > 0 {
				p.H = h
			}
			if err := parseNetPathPadShape(pm, &p, prefix); err != nil {
				return nil, err
			}
			pads = append(pads, p)
		}
	}
	return pads, nil
}

func parseNetPathLines(result map[string]any) ([]pcbTrack, []pcbArc, error) {
	lines, err := netPathRequiredSlice(result, "lines")
	if err != nil {
		return nil, nil, err
	}
	arcsRaw, err := netPathRequiredSlice(result, "arcs")
	if err != nil {
		return nil, nil, fmt.Errorf("%w (connector may be too old to expose arc data)", err)
	}
	arcsAvailable, ok := result["arcsAvailable"].(bool)
	if !ok || !arcsAvailable {
		return nil, nil, fmt.Errorf("result.arcsAvailable is not true; arc readback is unavailable and absence of arcs cannot be proven")
	}
	var tracks []pcbTrack
	for i, rl := range lines {
		m, ok := rl.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("lines[%d] is not an object", i)
		}
		prefix := fmt.Sprintf("lines[%d]", i)
		id := strings.TrimSpace(asString(m["primitiveId"]))
		if id == "" {
			return nil, nil, fmt.Errorf("%s.primitiveId is missing", prefix)
		}
		layer, err := netPathCopperLayerNumber(m, prefix)
		if err != nil {
			return nil, nil, err
		}
		x1, err := netPathNumber(m, "startX", prefix)
		if err != nil {
			return nil, nil, err
		}
		y1, err := netPathNumber(m, "startY", prefix)
		if err != nil {
			return nil, nil, err
		}
		x2, err := netPathNumber(m, "endX", prefix)
		if err != nil {
			return nil, nil, err
		}
		y2, err := netPathNumber(m, "endY", prefix)
		if err != nil {
			return nil, nil, err
		}
		w, err := netPathNumber(m, "lineWidth", prefix)
		if err != nil || w <= 0 {
			return nil, nil, fmt.Errorf("%s.lineWidth must be a finite positive number", prefix)
		}
		tracks = append(tracks, pcbTrack{ID: id, Net: asString(m["net"]), Layer: layer, X1: x1, Y1: y1, X2: x2, Y2: y2, Width: w})
	}
	var arcs []pcbArc
	for i, ra := range arcsRaw {
		m, ok := ra.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("arcs[%d] is not an object", i)
		}
		prefix := fmt.Sprintf("arcs[%d]", i)
		id := strings.TrimSpace(asString(m["primitiveId"]))
		if id == "" {
			return nil, nil, fmt.Errorf("%s.primitiveId is missing", prefix)
		}
		layer, err := netPathCopperLayerNumber(m, prefix)
		if err != nil {
			return nil, nil, err
		}
		x1, err := netPathNumber(m, "startX", prefix)
		if err != nil {
			return nil, nil, err
		}
		y1, err := netPathNumber(m, "startY", prefix)
		if err != nil {
			return nil, nil, err
		}
		x2, err := netPathNumber(m, "endX", prefix)
		if err != nil {
			return nil, nil, err
		}
		y2, err := netPathNumber(m, "endY", prefix)
		if err != nil {
			return nil, nil, err
		}
		w, err := netPathNumber(m, "lineWidth", prefix)
		if err != nil || w <= 0 {
			return nil, nil, fmt.Errorf("%s.lineWidth must be a finite positive number", prefix)
		}
		a, err := netPathNumber(m, "arcAngle", prefix)
		if err != nil || math.Abs(a) <= netPathGeomEps {
			return nil, nil, fmt.Errorf("%s.arcAngle must be a finite non-zero number", prefix)
		}
		arcs = append(arcs, pcbArc{ID: id, Net: asString(m["net"]), Layer: layer, X1: x1, Y1: y1, X2: x2, Y2: y2, Width: w, ArcAngle: a})
	}
	return tracks, arcs, nil
}

func parseNetPathVias(result map[string]any) ([]pcbViaP, error) {
	raw, err := netPathRequiredSlice(result, "vias")
	if err != nil {
		return nil, err
	}
	var vias []pcbViaP
	for i, rv := range raw {
		m, ok := rv.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("vias[%d] is not an object", i)
		}
		prefix := fmt.Sprintf("vias[%d]", i)
		id := strings.TrimSpace(asString(m["primitiveId"]))
		if id == "" {
			return nil, fmt.Errorf("%s.primitiveId is missing", prefix)
		}
		x, err := netPathNumber(m, "x", prefix)
		if err != nil {
			return nil, err
		}
		y, err := netPathNumber(m, "y", prefix)
		if err != nil {
			return nil, err
		}
		hole, err := netPathNumber(m, "holeDiameter", prefix)
		if err != nil || hole <= 0 {
			return nil, fmt.Errorf("%s.holeDiameter must be a finite positive number", prefix)
		}
		dia, err := netPathNumber(m, "diameter", prefix)
		if err != nil || dia <= 0 {
			return nil, fmt.Errorf("%s.diameter must be a finite positive number", prefix)
		}
		if dia <= hole {
			return nil, fmt.Errorf("%s.diameter must be greater than holeDiameter", prefix)
		}
		vias = append(vias, pcbViaP{ID: id, Net: asString(m["net"]), X: x, Y: y, Hole: hole, Dia: dia})
	}
	return vias, nil
}

func netPathRequiredSlice(result map[string]any, key string) ([]any, error) {
	v, ok := result[key]
	if !ok {
		return nil, fmt.Errorf("result.%s is missing; connectivity data is unavailable", key)
	}
	a, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("result.%s is not an array; connectivity data is unavailable", key)
	}
	return a, nil
}

func netPathOptionalFinite(v any) (float64, bool) {
	n, ok := asFloatOK(v)
	return n, ok && !math.IsNaN(n) && !math.IsInf(n, 0)
}

func netPathNumber(m map[string]any, key, prefix string) (float64, error) {
	n, ok := netPathOptionalFinite(m[key])
	if !ok {
		return 0, fmt.Errorf("%s.%s must be a finite number", prefix, key)
	}
	return n, nil
}

func netPathCopperLayerNumber(m map[string]any, prefix string) (int, error) {
	n, err := netPathNumber(m, "layer", prefix)
	if err != nil || n != math.Trunc(n) || !netPathCopperLayer(int(n)) {
		return 0, fmt.Errorf("%s.layer must be a copper layer id (TOP=1, BOTTOM=2, INNER=15..44)", prefix)
	}
	return int(n), nil
}

func parseNetPathPadShape(m map[string]any, p *pcbPadP, prefix string) error {
	if special, exists := m["specialPad"]; exists && special != nil {
		a, ok := special.([]any)
		if !ok {
			return fmt.Errorf("%s.specialPad is malformed", prefix)
		}
		if len(a) > 0 {
			p.SpecialPad = true
			p.ShapeIssue = "specialPad geometry is not supported by pcb net-path"
		}
	}
	raw, exists := m["shape"]
	if !exists || raw == nil {
		p.ShapeIssue = "shape field is missing (connector/readback capability unknown)"
		return nil
	}
	shape, ok := raw.([]any)
	if !ok || len(shape) < 2 {
		return fmt.Errorf("%s.shape is malformed", prefix)
	}
	kind, ok := shape[0].(string)
	if !ok || strings.TrimSpace(kind) == "" {
		return fmt.Errorf("%s.shape[0] must be a shape name", prefix)
	}
	p.Shape = strings.ToUpper(strings.TrimSpace(kind))
	positive := func(index int, name string) (float64, error) {
		if index >= len(shape) {
			return 0, fmt.Errorf("%s.shape %s is missing", prefix, name)
		}
		n, ok := netPathOptionalFinite(shape[index])
		if !ok || n <= 0 {
			return 0, fmt.Errorf("%s.shape %s must be a finite positive number", prefix, name)
		}
		return n, nil
	}
	switch p.Shape {
	case "RECT", "ELLIPSE", "OVAL":
		w, err := positive(1, "width")
		if err != nil {
			return err
		}
		h, err := positive(2, "height")
		if err != nil {
			return err
		}
		p.ShapeW, p.ShapeH = w, h
		if p.Shape == "RECT" {
			if len(shape) < 4 {
				return fmt.Errorf("%s.shape RECT corner radius is missing", prefix)
			}
			r, ok := netPathOptionalFinite(shape[3])
			if !ok || r < 0 || r > math.Min(w, h)/2+netPathGeomEps {
				return fmt.Errorf("%s.shape RECT corner radius is invalid", prefix)
			}
			p.ShapeRound = math.Min(r, math.Min(w, h)/2)
		}
		p.ShapeOK = !p.SpecialPad
	case "NGON":
		d, err := positive(1, "diameter")
		if err != nil {
			return err
		}
		sides, err := positive(2, "side count")
		if err != nil {
			return err
		}
		if sides != math.Trunc(sides) || sides < 3 {
			return fmt.Errorf("%s.shape NGON side count must be an integer >= 3", prefix)
		}
		p.ShapeW, p.ShapeH, p.ShapeSides = d, d, int(sides)
		p.ShapeOK = !p.SpecialPad
	case "POLYGON":
		// Validate the tuple container while keeping the geometry fail-closed.
		if shape[1] == nil {
			return fmt.Errorf("%s.shape POLYGON source is missing", prefix)
		}
		p.ShapeIssue = "POLYGON pad geometry is not supported by pcb net-path"
	case "":
		return fmt.Errorf("%s.shape name is empty", prefix)
	default:
		p.ShapeIssue = fmt.Sprintf("pad shape %q is not supported by pcb net-path", p.Shape)
	}
	return nil
}

func resolveNetPathWaypoints(pads []pcbPadP, opts pcbNetPathOptions) ([]string, string, error) {
	raw := append([]string{opts.From}, opts.Through...)
	raw = append(raw, opts.To)
	if len(raw) < 2 {
		return nil, "", fmt.Errorf("--from and --to are required")
	}
	refs := make([]string, 0, len(raw))
	seen := map[string]bool{}
	net := ""
	for _, r := range raw {
		d, n, err := splitPadRef(r)
		if err != nil {
			return nil, "", err
		}
		matches := make([]pcbPadP, 0, 1)
		for _, p := range pads {
			if strings.EqualFold(strings.TrimSpace(p.Designator), d) && strings.EqualFold(strings.TrimSpace(p.Number), n) {
				matches = append(matches, p)
			}
		}
		if len(matches) == 0 {
			return nil, "", fmt.Errorf("pad %s.%s was not found", d, n)
		}
		if len(matches) > 1 {
			return nil, "", fmt.Errorf("pad %s.%s is ambiguous (%d matches)", d, n, len(matches))
		}
		canon := strings.TrimSpace(matches[0].Designator) + "." + strings.TrimSpace(matches[0].Number)
		key := strings.ToLower(canon)
		if seen[key] {
			return nil, "", fmt.Errorf("waypoint %s is repeated", canon)
		}
		seen[key] = true
		pn := strings.TrimSpace(matches[0].Net)
		if pn == "" {
			return nil, "", fmt.Errorf("pad %s has no net; a copper path cannot be proven", canon)
		}
		if net == "" {
			net = pn
		} else if pn != net {
			return nil, "", fmt.Errorf("waypoint nets differ: %s is %q, expected %q", canon, pn, net)
		}
		refs = append(refs, canon)
	}
	if want := strings.TrimSpace(opts.Net); want != "" && want != net {
		return nil, "", fmt.Errorf("--net %q does not match waypoint net %q", want, net)
	}
	return refs, net, nil
}

func splitPadRef(s string) (string, string, error) {
	s = strings.TrimSpace(s)
	i := strings.LastIndexAny(s, ".:")
	if i <= 0 || i == len(s)-1 {
		return "", "", fmt.Errorf("invalid pad reference %q; use DESIGNATOR.PAD (for example U2.3)", s)
	}
	d, n := strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
	if d == "" || n == "" {
		return "", "", fmt.Errorf("invalid pad reference %q; use DESIGNATOR.PAD", s)
	}
	return d, n, nil
}

func analyzePcbNetPath(pads []pcbPadP, tracks []pcbTrack, arcs []pcbArc, vias []pcbViaP, opts pcbNetPathOptions) (pcbNetPathReport, error) {
	refs, net, err := resolveNetPathWaypoints(pads, opts)
	if err != nil {
		return pcbNetPathReport{}, err
	}
	refSet := make(map[string]bool, len(refs))
	for _, ref := range refs {
		refSet[strings.ToLower(ref)] = true
	}
	var unknownSameNet []string
	for _, p := range pads {
		if p.Net != net || p.ShapeOK {
			continue
		}
		ref := netPathPadLabel(p)
		issue := p.ShapeIssue
		if issue == "" {
			issue = "pad shape could not be proven"
		}
		if refSet[strings.ToLower(ref)] {
			return pcbNetPathReport{}, fmt.Errorf("path is unknown: waypoint %s has unsupported copper geometry: %s", ref, issue)
		}
		unknownSameNet = append(unknownSameNet, ref+" ("+issue+")")
	}
	if len(refs) > 2 {
		if reason, err := orderedNetPathCopperAmbiguity(tracks, arcs, vias, net, opts.Layer); err != nil {
			return pcbNetPathReport{}, fmt.Errorf("ordered waypoint topology is unknown: %w", err)
		} else if reason != "" {
			return pcbNetPathReport{}, fmt.Errorf("ordered waypoint topology is unknown: %s; physical copper union is not canonicalized", reason)
		}
	}
	rep := pcbNetPathReport{
		Net: net, From: refs[0], To: refs[len(refs)-1], WaypointOrder: refs,
		Scope:          "placed pad copper + listed tracks/arcs/vias; copper pours, fills, and PLANE connectivity excluded",
		ExcludedCopper: []string{"pours", "filled regions", "PLANE layers"},
		Limitations: []string{
			"does not infer connectivity through copper pours, filled regions, or PLANE layers",
			"arc primitives are continuous between their endpoints, but contacts to the interior of an arc are not inferred",
			"rotated RECT/rounded RECT and OVAL contacts use shape geometry; ELLIPSE, NGON, and pad-to-pad contacts use conservative inscribed geometry that may miss edge landings but cannot invent them",
		},
	}
	if opts.Layer != nil {
		layer := *opts.Layer
		rep.RequestedLayer = &layer
		rep.Scope = fmt.Sprintf("placed pad copper + listed tracks/arcs constrained to layer %d; physical vias, copper pours, fills, and PLANE connectivity excluded", layer)
		rep.ExcludedCopper = append(rep.ExcludedCopper, "physical vias (--layer constraint)")
		rep.ExcludedCopper = append(rep.ExcludedCopper, fmt.Sprintf("tracks/arcs outside layer %d", layer))
		rep.Limitations = append(rep.Limitations, fmt.Sprintf("--layer %d builds a layer-restricted graph; physical vias and all other-layer tracks/arcs are excluded", layer))
	} else {
		rep.Limitations = append(rep.Limitations, "vias are modeled as through vias because pcb.via.list exposes no blind/buried layer pair")
		rep.Limitations = append(rep.Limitations, "direct via-on-pad overlap is not treated as connected; a listed track/arc stub must bridge the pad and via")
	}
	if len(refs) > 2 {
		rep.Through = append([]string(nil), refs[1:len(refs)-1]...)
	}

	nodes, byRef := buildNetPathNodes(pads, tracks, arcs, vias, net, opts.Layer)
	adj := make([][]int, len(nodes))
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			if netPathNodesTouch(nodes[i], nodes[j], opts.Layer) {
				adj[i] = append(adj[i], j)
				adj[j] = append(adj[j], i)
			}
		}
	}

	waypointNodes := make([]int, len(refs))
	for i, ref := range refs {
		idx, ok := byRef[strings.ToLower(ref)]
		if !ok {
			return pcbNetPathReport{}, fmt.Errorf("internal pad graph is missing waypoint %s", ref)
		}
		waypointNodes[i] = idx
	}
	var combined []int
	if len(waypointNodes) == 2 {
		combined = bfsNetPath(adj, waypointNodes[0], waypointNodes[1])
	} else {
		var exhausted bool
		combined, exhausted = orderedSimpleNetPath(adj, waypointNodes)
		if exhausted {
			return pcbNetPathReport{}, fmt.Errorf("ordered waypoint search exceeded its safety bound; path is unknown, not failed")
		}
	}
	rep.Connected = len(combined) > 0
	if !rep.Connected {
		rep.Reason = "no single continuous listed-copper path visits the requested pads in order without reusing copper primitives"
		if opts.Layer != nil {
			rep.Reason = fmt.Sprintf("no single continuous path constrained to layer %d visits the requested pads in order without reusing copper primitives", *opts.Layer)
		}
	}
	positions := map[int]int{}
	for i, n := range combined {
		positions[n] = i
	}
	for i := 0; i+1 < len(refs); i++ {
		var path []int
		if rep.Connected {
			start, end := positions[waypointNodes[i]], positions[waypointNodes[i+1]]
			path = combined[start : end+1]
		} else {
			// Diagnostic only: report whether each neighboring pair is reachable on
			// its own. Overall remains FAIL unless one simple path carries every
			// waypoint in order; concatenated backtracking is not accepted as proof.
			path = bfsNetPath(adj, waypointNodes[i], waypointNodes[i+1])
		}
		leg := pcbNetPathLeg{From: refs[i], To: refs[i+1], Connected: len(path) > 0}
		if len(path) == 0 {
			leg.Reason = "no continuous path in listed track/arc/via copper (pours and PLANE connectivity are excluded)"
			if opts.Layer != nil {
				leg.Reason = fmt.Sprintf("no continuous path in listed copper constrained to layer %d (physical vias, pours, and PLANE connectivity are excluded)", *opts.Layer)
			}
		} else {
			leg.Path = netPathSteps(nodes, path)
			fillNetPathStats(&leg.Layers, nil, &leg.WidthsMil, &leg.MinWidth, &leg.MaxWidth, &leg.ViaCount, &leg.Primitives, nodes, path)
			if !rep.Connected {
				leg.Reason = "this leg is individually reachable, but the full waypoint order has no single non-repeating copper path"
			}
		}
		rep.Legs = append(rep.Legs, leg)
	}
	if rep.Connected {
		rep.Path = netPathSteps(nodes, combined)
		fillNetPathStats(&rep.Layers, &rep.LayerSequence, &rep.WidthsMil, &rep.MinWidth, &rep.MaxWidth, &rep.ViaCount, &rep.Primitives, nodes, combined)
	}
	if len(unknownSameNet) > 0 {
		rep.Limitations = append(rep.Limitations, fmt.Sprintf("%d unsupported same-net pad(s) were excluded from the proof graph: %s", len(unknownSameNet), strings.Join(unknownSameNet, "; ")))
		if !rep.Connected {
			return pcbNetPathReport{}, fmt.Errorf("path is unknown, not failed: unsupported same-net pad geometry could change reachability: %s", strings.Join(unknownSameNet, "; "))
		}
	}
	return rep, nil
}

type netPathPoint struct{ x, y float64 }

// netPathCopperCurve is a routed primitive's centerline. Tracks are exact two-
// point curves. Arcs are flattened with a bounded sagitta error so the ordered
// proof can detect copper-area overlap instead of comparing primitive IDs or
// chords. ApproxErr is the maximum centerline deviation in mil.
type netPathCopperCurve struct {
	kind      string
	id        string
	layer     int
	width     float64
	points    []netPathPoint
	approxErr float64
}

// orderedNetPathCopperAmbiguity protects --through's non-repeating-primitive
// proof. Two primitive IDs are not independent graph edges when their physical
// copper overlaps away from one ordinary endpoint/T junction. In that case a
// graph path can manufacture a waypoint order that the copper topology does not
// establish, so the result must be unknown rather than PASS or FAIL.
func orderedNetPathCopperAmbiguity(tracks []pcbTrack, arcs []pcbArc, vias []pcbViaP, net string, requestedLayer *int) (string, error) {
	curves := make([]netPathCopperCurve, 0, len(tracks)+len(arcs))
	for _, t := range tracks {
		if t.Net != net || (requestedLayer != nil && t.Layer != *requestedLayer) {
			continue
		}
		if math.Hypot(t.X2-t.X1, t.Y2-t.Y1) <= netPathGeomEps {
			return "", fmt.Errorf("track %s has a degenerate centerline that cannot be normalized", t.ID)
		}
		curves = append(curves, netPathCopperCurve{
			kind: "track", id: t.ID, layer: t.Layer, width: t.Width,
			points: []netPathPoint{{t.X1, t.Y1}, {t.X2, t.Y2}},
		})
	}
	for _, a := range arcs {
		if a.Net != net || (requestedLayer != nil && a.Layer != *requestedLayer) {
			continue
		}
		points, approximation, err := flattenNetPathArc(a)
		if err != nil {
			return "", fmt.Errorf("arc %s cannot be normalized: %w", a.ID, err)
		}
		curves = append(curves, netPathCopperCurve{
			kind: "arc", id: a.ID, layer: a.Layer, width: a.Width,
			points: points, approxErr: approximation,
		})
	}

	for i := 0; i < len(curves); i++ {
		for j := i + 1; j < len(curves); j++ {
			a, b := curves[i], curves[j]
			if a.layer != b.layer || !netPathCurveBoundsMayOverlap(a, b) {
				continue
			}
			overlaps, ordinaryJunction, detail, err := netPathCurvePairOverlap(a, b)
			if err != nil {
				return "", fmt.Errorf("%s %s and %s %s: %w", a.kind, a.id, b.kind, b.id, err)
			}
			if overlaps && !ordinaryJunction {
				return fmt.Sprintf("same-layer %s %s and %s %s have %s", a.kind, a.id, b.kind, b.id, detail), nil
			}
		}
	}

	if requestedLayer == nil {
		var selected []pcbViaP
		for _, v := range vias {
			if v.Net == net {
				selected = append(selected, v)
			}
		}
		for i := 0; i < len(selected); i++ {
			for j := i + 1; j < len(selected); j++ {
				a, b := selected[i], selected[j]
				distance := math.Hypot(a.X-b.X, a.Y-b.Y)
				limit := a.Dia/2 + b.Dia/2
				if distance < limit-netPathGeomEps {
					return fmt.Sprintf("through vias %s and %s have overlapping copper annuli", a.ID, b.ID), nil
				}
				if distance <= limit+netPathGeomEps {
					return "", fmt.Errorf("through vias %s and %s are tangent within %.3g mil geometry tolerance", a.ID, b.ID, netPathGeomEps)
				}
			}
		}
	}
	return "", nil
}

// flattenNetPathArc follows the connector's verified ARC convention: a positive
// sweep advances counter-clockwise from start to end. Chords are short enough
// that their maximum sagitta is <= netPathGeomEps/4. A hard segment cap prevents
// a huge/near-full circle from silently degrading either precision or runtime.
func flattenNetPathArc(a pcbArc) ([]netPathPoint, float64, error) {
	absSweep := math.Abs(a.ArcAngle)
	chordX, chordY := a.X2-a.X1, a.Y2-a.Y1
	chord := math.Hypot(chordX, chordY)
	if absSweep <= netPathGeomEps || absSweep >= 360-netPathGeomEps || chord <= netPathGeomEps {
		return nil, 0, fmt.Errorf("requires distinct endpoints and |arcAngle| between 0 and 360 degrees")
	}
	sineHalf := math.Sin(absSweep * math.Pi / 360)
	if sineHalf <= 0 || math.IsNaN(sineHalf) || math.IsInf(sineHalf, 0) {
		return nil, 0, fmt.Errorf("invalid sweep %.9g degrees", a.ArcAngle)
	}
	radius := chord / (2 * sineHalf)
	midX, midY := (a.X1+a.X2)/2, (a.Y1+a.Y2)/2
	offsetSq := radius*radius - chord*chord/4
	if offsetSq < -netPathGeomEps*netPathGeomEps {
		return nil, 0, fmt.Errorf("radius reconstruction is inconsistent")
	}
	centerOffset := math.Sqrt(math.Max(0, offsetSq))
	normalX, normalY := -chordY/chord, chordX/chord
	type centerCandidate struct{ mismatch, x, y float64 }
	best := centerCandidate{mismatch: math.Inf(1)}
	for _, sign := range []float64{1, -1} {
		cx, cy := midX+sign*centerOffset*normalX, midY+sign*centerOffset*normalY
		start := math.Atan2(a.Y1-cy, a.X1-cx) * 180 / math.Pi
		end := math.Atan2(a.Y2-cy, a.X2-cx) * 180 / math.Pi
		directed := end - start
		if a.ArcAngle < 0 {
			directed = start - end
		}
		directed = math.Mod(directed, 360)
		if directed < 0 {
			directed += 360
		}
		candidate := centerCandidate{mismatch: math.Abs(directed - absSweep), x: cx, y: cy}
		if candidate.mismatch < best.mismatch {
			best = candidate
		}
	}
	if best.mismatch > 1e-4 || math.IsInf(radius, 0) || math.IsNaN(radius) {
		return nil, 0, fmt.Errorf("directed sweep does not match endpoints (mismatch %.6g degrees)", best.mismatch)
	}

	const targetSagitta = netPathGeomEps / 4
	cosArg := 1 - targetSagitta/radius
	cosArg = math.Max(-1, math.Min(1, cosArg))
	maxStep := 2 * math.Acos(cosArg)
	if maxStep <= 0 || math.IsNaN(maxStep) {
		return nil, 0, fmt.Errorf("cannot establish a bounded polyline approximation")
	}
	steps := int(math.Ceil(absSweep * math.Pi / 180 / maxStep))
	if steps < 1 {
		steps = 1
	}
	const maxArcSegments = 4096
	if steps > maxArcSegments {
		return nil, 0, fmt.Errorf("needs %d segments for %.4g mil error bound (limit %d)", steps, targetSagitta, maxArcSegments)
	}
	startRadians := math.Atan2(a.Y1-best.y, a.X1-best.x)
	points := make([]netPathPoint, 0, steps+1)
	points = append(points, netPathPoint{a.X1, a.Y1})
	for step := 1; step < steps; step++ {
		angle := startRadians + a.ArcAngle*math.Pi/180*float64(step)/float64(steps)
		points = append(points, netPathPoint{best.x + radius*math.Cos(angle), best.y + radius*math.Sin(angle)})
	}
	points = append(points, netPathPoint{a.X2, a.Y2})
	actualSagitta := radius * (1 - math.Cos(absSweep*math.Pi/180/float64(steps)/2))
	return points, actualSagitta, nil
}

func netPathCurveBoundsMayOverlap(a, b netPathCopperCurve) bool {
	aminX, aminY, amaxX, amaxY := netPathCurveBounds(a)
	bminX, bminY, bmaxX, bmaxY := netPathCurveBounds(b)
	margin := a.width/2 + b.width/2 + a.approxErr + b.approxErr + netPathGeomEps
	return aminX <= bmaxX+margin && bminX <= amaxX+margin && aminY <= bmaxY+margin && bminY <= amaxY+margin
}

func netPathCurveBounds(c netPathCopperCurve) (minX, minY, maxX, maxY float64) {
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)
	for _, p := range c.points {
		minX, minY = math.Min(minX, p.x), math.Min(minY, p.y)
		maxX, maxY = math.Max(maxX, p.x), math.Max(maxY, p.y)
	}
	return
}

func netPathCurvePairOverlap(a, b netPathCopperCurve) (overlaps, ordinaryJunction bool, detail string, err error) {
	pairs := (len(a.points) - 1) * (len(b.points) - 1)
	const maxCurveSegmentPairs = 2_000_000
	if pairs > maxCurveSegmentPairs {
		return false, false, "", fmt.Errorf("requires %d segment comparisons (limit %d)", pairs, maxCurveSegmentPairs)
	}
	minDistance := math.Inf(1)
	properCross := false
	positiveCollinearOverlap := false
	for i := 0; i+1 < len(a.points); i++ {
		ap, aq := a.points[i], a.points[i+1]
		for j := 0; j+1 < len(b.points); j++ {
			bp, bq := b.points[j], b.points[j+1]
			minDistance = math.Min(minDistance, segSegDist(ap.x, ap.y, aq.x, aq.y, bp.x, bp.y, bq.x, bq.y))
			if segSegCross(ap.x, ap.y, aq.x, aq.y, bp.x, bp.y, bq.x, bq.y) {
				properCross = true
			}
			if netPathPositiveCollinearOverlap(ap, aq, bp, bq) {
				positiveCollinearOverlap = true
			}
		}
	}
	radius := a.width/2 + b.width/2
	errorBound := a.approxErr + b.approxErr + netPathGeomEps
	if minDistance > radius+errorBound {
		return false, false, "", nil
	}

	junctions := netPathEndpointJunctions(a, b)
	if len(junctions) == 1 && !properCross && !positiveCollinearOverlap {
		return true, true, "one endpoint/T junction", nil
	}
	if positiveCollinearOverlap {
		return true, false, "positive-length centerline overlap", nil
	}
	if properCross {
		return true, false, "interior centerline crossing with overlapping copper", nil
	}
	if len(junctions) > 1 {
		return true, false, fmt.Sprintf("overlapping copper at %d distinct endpoint/T junctions", len(junctions)), nil
	}
	if minDistance >= radius-errorBound {
		return true, false, "copper contact within the geometry-error band", nil
	}
	return true, false, "copper-area overlap without a single endpoint/T junction", nil
}

func netPathEndpointJunctions(a, b netPathCopperCurve) []netPathPoint {
	tolerance := netPathGeomEps + a.approxErr + b.approxErr
	var out []netPathPoint
	add := func(p netPathPoint) {
		for _, q := range out {
			if math.Hypot(p.x-q.x, p.y-q.y) <= 2*tolerance {
				return
			}
		}
		out = append(out, p)
	}
	for _, p := range []netPathPoint{a.points[0], a.points[len(a.points)-1]} {
		if netPathPointCurveDistance(p, b) <= tolerance {
			add(p)
		}
	}
	for _, p := range []netPathPoint{b.points[0], b.points[len(b.points)-1]} {
		if netPathPointCurveDistance(p, a) <= tolerance {
			add(p)
		}
	}
	return out
}

func netPathPointCurveDistance(p netPathPoint, c netPathCopperCurve) float64 {
	best := math.Inf(1)
	for i := 0; i+1 < len(c.points); i++ {
		q, r := c.points[i], c.points[i+1]
		best = math.Min(best, segPtDist(p.x, p.y, q.x, q.y, r.x, r.y))
	}
	return best
}

func netPathPositiveCollinearOverlap(a, b, c, d netPathPoint) bool {
	abx, aby := b.x-a.x, b.y-a.y
	cdx, cdy := d.x-c.x, d.y-c.y
	abLen, cdLen := math.Hypot(abx, aby), math.Hypot(cdx, cdy)
	if abLen <= netPathGeomEps || cdLen <= netPathGeomEps {
		return false
	}
	if math.Abs(abx*cdy-aby*cdx) > netPathGeomEps*abLen*cdLen {
		return false
	}
	if math.Abs(abx*(c.y-a.y)-aby*(c.x-a.x)) > netPathGeomEps*abLen {
		return false
	}
	ux, uy := abx/abLen, aby/abLen
	c0 := (c.x-a.x)*ux + (c.y-a.y)*uy
	c1 := (d.x-a.x)*ux + (d.y-a.y)*uy
	lo := math.Max(0, math.Min(c0, c1))
	hi := math.Min(abLen, math.Max(c0, c1))
	return hi-lo > netPathGeomEps
}

func buildNetPathNodes(pads []pcbPadP, tracks []pcbTrack, arcs []pcbArc, vias []pcbViaP, net string, requestedLayer *int) ([]pcbNetPathNode, map[string]int) {
	var nodes []pcbNetPathNode
	var ps []pcbPadP
	for _, p := range pads {
		if p.Net == net && p.ShapeOK {
			ps = append(ps, p)
		}
	}
	sort.Slice(ps, func(i, j int) bool {
		return strings.ToLower(netPathPadLabel(ps[i])) < strings.ToLower(netPathPadLabel(ps[j]))
	})
	byRef := map[string]int{}
	for _, p := range ps {
		ref := netPathPadLabel(p)
		n := pcbNetPathNode{kind: "pad", id: p.ID, ref: ref, net: p.Net, layer: p.Layer, x: p.X, y: p.Y, w: p.ShapeW, h: p.ShapeH, rotation: p.Rotation, shape: p.Shape, shapeRound: p.ShapeRound, shapeSides: p.ShapeSides}
		if p.Designator != "" && p.Number != "" {
			byRef[strings.ToLower(ref)] = len(nodes)
		}
		nodes = append(nodes, n)
	}
	ts := append([]pcbTrack(nil), tracks...)
	sort.Slice(ts, func(i, j int) bool { return ts[i].ID < ts[j].ID })
	for _, t := range ts {
		if t.Net == net && (requestedLayer == nil || t.Layer == *requestedLayer) {
			nodes = append(nodes, pcbNetPathNode{kind: "track", id: t.ID, net: t.Net, layer: t.Layer, width: t.Width, x1: t.X1, y1: t.Y1, x2: t.X2, y2: t.Y2})
		}
	}
	as := append([]pcbArc(nil), arcs...)
	sort.Slice(as, func(i, j int) bool { return as[i].ID < as[j].ID })
	for _, a := range as {
		if a.Net == net && (requestedLayer == nil || a.Layer == *requestedLayer) {
			nodes = append(nodes, pcbNetPathNode{kind: "arc", id: a.ID, net: a.Net, layer: a.Layer, width: a.Width, x1: a.X1, y1: a.Y1, x2: a.X2, y2: a.Y2, arcAngle: a.ArcAngle})
		}
	}
	vs := append([]pcbViaP(nil), vias...)
	sort.Slice(vs, func(i, j int) bool { return vs[i].ID < vs[j].ID })
	for _, v := range vs {
		if v.Net == net && requestedLayer == nil {
			nodes = append(nodes, pcbNetPathNode{kind: "via", id: v.ID, net: v.Net, x: v.X, y: v.Y, dia: v.Dia})
		}
	}
	return nodes, byRef
}

func netPathPadLabel(p pcbPadP) string {
	if strings.TrimSpace(p.Designator) != "" && strings.TrimSpace(p.Number) != "" {
		return strings.TrimSpace(p.Designator) + "." + strings.TrimSpace(p.Number)
	}
	return p.ID
}

func netPathNodesTouch(a, b pcbNetPathNode, requestedLayer *int) bool {
	if a.net == "" || a.net != b.net {
		return false
	}
	if requestedLayer != nil {
		if a.kind == "via" || b.kind == "via" {
			return false
		}
		if a.kind == "pad" && !padLayerMatches(a.layer, *requestedLayer) {
			return false
		}
		if b.kind == "pad" && !padLayerMatches(b.layer, *requestedLayer) {
			return false
		}
	}
	if a.kind > b.kind {
		return netPathNodesTouch(b, a, requestedLayer)
	}
	switch a.kind + ":" + b.kind {
	case "arc:arc":
		if a.layer != b.layer {
			return false
		}
		return endpointsNear(a, b, a.width/2+b.width/2+netPathGeomEps)
	case "arc:pad":
		if !padLayerMatches(b.layer, a.layer) {
			return false
		}
		return pointTouchesPad(a.x1, a.y1, b, a.width/2) || pointTouchesPad(a.x2, a.y2, b, a.width/2)
	case "arc:track":
		if a.layer != b.layer {
			return false
		}
		r := a.width/2 + b.width/2 + netPathGeomEps
		return segPtDist(a.x1, a.y1, b.x1, b.y1, b.x2, b.y2) <= r || segPtDist(a.x2, a.y2, b.x1, b.y1, b.x2, b.y2) <= r
	case "arc:via":
		r := a.width/2 + b.dia/2 + netPathGeomEps
		return math.Hypot(a.x1-b.x, a.y1-b.y) <= r || math.Hypot(a.x2-b.x, a.y2-b.y) <= r
	case "pad:pad":
		if !padLayersCompatible(a.layer, b.layer) {
			return false
		}
		return padsTouch(a, b)
	case "pad:track":
		if !padLayerMatches(a.layer, b.layer) {
			return false
		}
		return trackTouchesPad(b, a)
	case "pad:via":
		// The host does not register a via merely overlapping a pad as connected
		// (live-verified via-in-pad behavior). Require a listed track/arc stub to
		// bridge pad↔via; otherwise this command would fabricate continuity.
		return false
	case "track:track":
		if a.layer != b.layer {
			return false
		}
		return segSegDist(a.x1, a.y1, a.x2, a.y2, b.x1, b.y1, b.x2, b.y2) <= a.width/2+b.width/2+netPathGeomEps
	case "track:via":
		return segPtDist(b.x, b.y, a.x1, a.y1, a.x2, a.y2) <= a.width/2+b.dia/2+netPathGeomEps
	case "via:via":
		return math.Hypot(a.x-b.x, a.y-b.y) <= a.dia/2+b.dia/2+netPathGeomEps
	}
	return false
}

func padLayerMatches(padLayer, copperLayer int) bool {
	return padLayer == pcbLayerMulti || padLayer == copperLayer
}
func padLayersCompatible(a, b int) bool { return a == pcbLayerMulti || b == pcbLayerMulti || a == b }
func netPathCopperLayer(layer int) bool {
	return layer == 1 || layer == 2 || (layer >= 15 && layer <= 44)
}

func trackTouchesPad(track, pad pcbNetPathNode) bool {
	x1, y1 := netPathPadLocal(pad, track.x1, track.y1)
	x2, y2 := netPathPadLocal(pad, track.x2, track.y2)
	r := track.width/2 + netPathGeomEps
	switch pad.shape {
	case "RECT":
		corner := pad.shapeRound
		return rectSegDist(-pad.w/2+corner, -pad.h/2+corner, pad.w/2-corner, pad.h/2-corner, x1, y1, x2, y2) <= r+corner
	case "OVAL":
		px1, py1, px2, py2, radius := netPathOvalSpine(pad)
		return segSegDist(x1, y1, x2, y2, px1, py1, px2, py2) <= r+radius
	case "ELLIPSE", "NGON":
		// Use a circle fully contained in the copper. This may miss a legal edge
		// landing on a non-circular ellipse/polygon, but can never invent one.
		return segPtDist(0, 0, x1, y1, x2, y2) <= r+netPathPadInnerRadius(pad)
	}
	return false
}

func pointTouchesPad(x, y float64, pad pcbNetPathNode, radius float64) bool {
	lx, ly := netPathPadLocal(pad, x, y)
	r := radius + netPathGeomEps
	switch pad.shape {
	case "RECT":
		corner := pad.shapeRound
		return rectPtDist(-pad.w/2+corner, -pad.h/2+corner, pad.w/2-corner, pad.h/2-corner, lx, ly) <= r+corner
	case "OVAL":
		x1, y1, x2, y2, pr := netPathOvalSpine(pad)
		return segPtDist(lx, ly, x1, y1, x2, y2) <= r+pr
	case "ELLIPSE", "NGON":
		return math.Hypot(lx, ly) <= r+netPathPadInnerRadius(pad)
	}
	return false
}

func padsTouch(a, b pcbNetPathNode) bool {
	// Direct pad-to-pad contact is uncommon. Prove only overlap of inscribed
	// circles, a conservative subset valid for every supported pad shape.
	return math.Hypot(a.x-b.x, a.y-b.y) <= netPathPadInnerRadius(a)+netPathPadInnerRadius(b)+netPathGeomEps
}

func netPathPadLocal(pad pcbNetPathNode, x, y float64) (float64, float64) {
	r := -pad.rotation * math.Pi / 180
	c, s := math.Cos(r), math.Sin(r)
	dx, dy := x-pad.x, y-pad.y
	return dx*c - dy*s, dx*s + dy*c
}

func netPathPadInnerRadius(pad pcbNetPathNode) float64 {
	switch pad.shape {
	case "NGON":
		if pad.shapeSides >= 3 {
			return pad.w / 2 * math.Cos(math.Pi/float64(pad.shapeSides))
		}
	case "RECT", "ELLIPSE", "OVAL":
		return math.Min(pad.w, pad.h) / 2
	}
	return 0
}

func netPathOvalSpine(pad pcbNetPathNode) (x1, y1, x2, y2, radius float64) {
	radius = math.Min(pad.w, pad.h) / 2
	if pad.w >= pad.h {
		half := (pad.w - pad.h) / 2
		return -half, 0, half, 0, radius
	}
	half := (pad.h - pad.w) / 2
	return 0, -half, 0, half, radius
}

func endpointsNear(a, b pcbNetPathNode, radius float64) bool {
	for _, p := range [][2]float64{{a.x1, a.y1}, {a.x2, a.y2}} {
		for _, q := range [][2]float64{{b.x1, b.y1}, {b.x2, b.y2}} {
			if math.Hypot(p[0]-q[0], p[1]-q[1]) <= radius {
				return true
			}
		}
	}
	return false
}

func bfsNetPath(adj [][]int, start, goal int) []int {
	if start == goal {
		return []int{start}
	}
	prev := make([]int, len(adj))
	for i := range prev {
		prev[i] = -2
	}
	prev[start] = -1
	q := []int{start}
	for len(q) > 0 {
		v := q[0]
		q = q[1:]
		for _, n := range adj[v] {
			if prev[n] != -2 {
				continue
			}
			prev[n] = v
			if n == goal {
				var rev []int
				for at := goal; at >= 0; at = prev[at] {
					rev = append(rev, at)
				}
				for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
					rev[i], rev[j] = rev[j], rev[i]
				}
				return rev
			}
			q = append(q, n)
		}
	}
	return nil
}

// orderedSimpleNetPath finds one non-repeating graph path that visits every
// requested pad in order. Checking each leg independently is insufficient on an
// undirected net: any connected branch can be walked in arbitrary order by
// backtracking, which would falsely "prove" a daisy-chain topology. The bounded
// DFS rejects future waypoints before their turn and never reuses a primitive.
func orderedSimpleNetPath(adj [][]int, waypoints []int) ([]int, bool) {
	if len(waypoints) < 2 {
		return nil, false
	}
	required := make(map[int]int, len(waypoints))
	for i, n := range waypoints {
		required[n] = i
	}
	dists := make([][]int, len(waypoints))
	for i := 1; i < len(waypoints); i++ {
		dists[i] = netPathDistances(adj, waypoints[i])
	}
	visited := make([]bool, len(adj))
	visited[waypoints[0]] = true
	path := []int{waypoints[0]}
	const maxStates = 500000
	states := 0
	exhausted := false
	var search func(node, next int) bool
	search = func(node, next int) bool {
		states++
		if states > maxStates {
			exhausted = true
			return false
		}
		if next < len(waypoints) && node == waypoints[next] {
			next++
		}
		if next == len(waypoints) {
			return true
		}
		candidates := make([]int, 0, len(adj[node]))
		for _, n := range adj[node] {
			if visited[n] {
				continue
			}
			if pos, isWaypoint := required[n]; isWaypoint && pos > next {
				continue // encountering a later waypoint now would violate order
			}
			candidates = append(candidates, n)
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			di, dj := dists[next][candidates[i]], dists[next][candidates[j]]
			if di != dj {
				return di < dj
			}
			return candidates[i] < candidates[j]
		})
		for _, n := range candidates {
			visited[n] = true
			path = append(path, n)
			if search(n, next) {
				return true
			}
			path = path[:len(path)-1]
			visited[n] = false
			if exhausted {
				return false
			}
		}
		return false
	}
	if search(waypoints[0], 1) {
		return append([]int(nil), path...), false
	}
	return nil, exhausted
}

func netPathDistances(adj [][]int, goal int) []int {
	const unreachable = int(^uint(0) >> 1)
	d := make([]int, len(adj))
	for i := range d {
		d[i] = unreachable
	}
	d[goal] = 0
	q := []int{goal}
	for len(q) > 0 {
		v := q[0]
		q = q[1:]
		for _, n := range adj[v] {
			if d[n] != unreachable {
				continue
			}
			d[n] = d[v] + 1
			q = append(q, n)
		}
	}
	return d
}

func netPathSteps(nodes []pcbNetPathNode, path []int) []pcbNetPathStep {
	out := make([]pcbNetPathStep, 0, len(path))
	for _, i := range path {
		n := nodes[i]
		s := pcbNetPathStep{Kind: n.kind, PrimitiveID: n.id, Ref: n.ref, Layer: n.layer, WidthMil: n.width}
		switch n.kind {
		case "pad", "via":
			s.At = &pcbNetPathPoint{X: round2(n.x), Y: round2(n.y)}
		case "track", "arc":
			s.Start = &pcbNetPathPoint{X: round2(n.x1), Y: round2(n.y1)}
			s.End = &pcbNetPathPoint{X: round2(n.x2), Y: round2(n.y2)}
			s.ArcAngle = round2(n.arcAngle)
		}
		out = append(out, s)
	}
	return out
}

func fillNetPathStats(layers, sequence *[]int, widths *[]float64, minW, maxW **float64, viaCount, primitives *int, nodes []pcbNetPathNode, path []int) {
	layerSet := map[int]bool{}
	widthSet := map[float64]bool{}
	viaSet := map[string]bool{}
	primSet := map[string]bool{}
	var seq []int
	for _, i := range path {
		n := nodes[i]
		if n.kind == "track" || n.kind == "arc" {
			layerSet[n.layer] = true
			if len(seq) == 0 || seq[len(seq)-1] != n.layer {
				seq = append(seq, n.layer)
			}
			if n.width > 0 {
				widthSet[round2(n.width)] = true
			}
			primSet[n.kind+":"+n.id] = true
		} else if n.kind == "via" {
			viaSet[n.id] = true
			primSet[n.kind+":"+n.id] = true
		}
	}
	for l := range layerSet {
		*layers = append(*layers, l)
	}
	sort.Ints(*layers)
	if sequence != nil {
		*sequence = seq
	}
	for w := range widthSet {
		*widths = append(*widths, w)
	}
	sort.Float64s(*widths)
	if len(*widths) > 0 {
		a, b := (*widths)[0], (*widths)[len(*widths)-1]
		*minW = &a
		*maxW = &b
	}
	*viaCount = len(viaSet)
	*primitives = len(primSet)
}

func renderPcbNetPath(rep pcbNetPathReport, out io.Writer) {
	status := "PASS"
	if !rep.Connected {
		status = "FAIL"
	}
	fmt.Fprintf(out, "PCB net path: %s  net=%s  %s\n", status, rep.Net, strings.Join(rep.WaypointOrder, " -> "))
	fmt.Fprintf(out, "scope: %s\n", rep.Scope)
	if rep.Reason != "" {
		fmt.Fprintf(out, "reason: %s\n", rep.Reason)
	}
	if rep.RequestedLayer != nil {
		fmt.Fprintf(out, "requested layer: %s (physical vias excluded)\n", formatNetPathLayers([]int{*rep.RequestedLayer}))
	}
	if rep.Connected {
		fmt.Fprintf(out, "layers: %s; sequence: %s; widths: %s; vias: %d; copper primitives: %d\n", formatNetPathLayers(rep.Layers), formatNetPathLayers(rep.LayerSequence), formatNetPathWidths(rep.WidthsMil), rep.ViaCount, rep.Primitives)
	}
	for i, leg := range rep.Legs {
		if !leg.Connected {
			fmt.Fprintf(out, "leg %d FAIL  %s -> %s: %s\n", i+1, leg.From, leg.To, leg.Reason)
			continue
		}
		fmt.Fprintf(out, "leg %d PASS  %s -> %s  layers=%s widths=%s vias=%d\n", i+1, leg.From, leg.To, formatNetPathLayers(leg.Layers), formatNetPathWidths(leg.WidthsMil), leg.ViaCount)
		for _, s := range leg.Path {
			switch s.Kind {
			case "pad":
				fmt.Fprintf(out, "  PAD   %-12s L%d @ (%.2f, %.2f)\n", s.Ref, s.Layer, s.At.X, s.At.Y)
			case "via":
				fmt.Fprintf(out, "  VIA   %-12s @ (%.2f, %.2f)\n", s.PrimitiveID, s.At.X, s.At.Y)
			default:
				fmt.Fprintf(out, "  %-5s %-12s L%d %.2fmil (%.2f, %.2f) -> (%.2f, %.2f)\n", strings.ToUpper(s.Kind), s.PrimitiveID, s.Layer, s.WidthMil, s.Start.X, s.Start.Y, s.End.X, s.End.Y)
			}
		}
	}
	for _, l := range rep.Limitations {
		fmt.Fprintf(out, "limit: %s\n", l)
	}
}

func formatNetPathLayers(in []int) string {
	if len(in) == 0 {
		return "none"
	}
	parts := make([]string, len(in))
	for i, l := range in {
		switch l {
		case 1:
			parts[i] = "TOP(1)"
		case 2:
			parts[i] = "BOTTOM(2)"
		default:
			parts[i] = fmt.Sprintf("L%d", l)
		}
	}
	return strings.Join(parts, " -> ")
}
func formatNetPathWidths(in []float64) string {
	if len(in) == 0 {
		return "none"
	}
	parts := make([]string, len(in))
	for i, w := range in {
		parts[i] = fmt.Sprintf("%gmil", w)
	}
	return strings.Join(parts, ",")
}
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
