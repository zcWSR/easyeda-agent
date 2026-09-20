package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPcbLayoutPlanCommandRunsFullyOfflineAndWritesBundle(t *testing.T) {
	tmp := t.TempDir()
	boardPath := filepath.Join(tmp, "board.json")
	layoutPath := filepath.Join(tmp, "layout.json")
	outDir := filepath.Join(tmp, "out")
	snap := lpSnapshot(lpComp("p1", "LED1", 200, 200, 0, lpBBox(180, 180, 220, 220)))
	b, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(boardPath, b, 0o600); err != nil {
		t.Fatal(err)
	}
	input := `{"schemaVersion":1,"units":"mil","coordinateSemantic":"footprint-anchor","apply":{"project":"ceshi","doc":"pcb-1"},"modules":[{"id":"led","strategy":"rigid","copperPolicy":"ignore","anchorRef":"LED1","members":[{"ref":"LED1"}],"search":{"offsetsMil":[{"xMil":20,"yMil":0,"label":"right"}]}}]}`
	if err = os.WriteFile(layoutPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	cmd := newPcbLayoutPlanCmd(&stdout, &stderr)
	cmd.SetArgs([]string{"--from", layoutPath, "--board", boardPath, "--module", "led", "--candidates", "3", "--out", outDir})
	if err = cmd.Execute(); err != nil {
		t.Fatalf("execute: %v stderr=%s", err, stderr.String())
	}
	for _, name := range []string{"manifest.json", "candidate-01.json", "candidate-01.apply.json", "candidate-01.svg"} {
		if _, err = os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	applyRaw, err := os.ReadFile(filepath.Join(outDir, "candidate-01.apply.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pb playbook
	if err = json.Unmarshal(applyRaw, &pb); err != nil {
		t.Fatal(err)
	}
	if len(pb.Steps) != 2 || pb.Steps[0].Action != "pcb.component.modify" || pb.Steps[1].Action != "pcb.save" {
		t.Fatalf("apply=%+v", pb.Steps)
	}
	if !stringContains(pb.Meta.Description, "boardSha256=") || !stringContains(pb.Meta.Description, "layoutSha256=") || !stringContains(pb.Meta.Description, "sha256(raw-file-bytes)") {
		t.Fatalf("apply provenance missing: %q", pb.Meta.Description)
	}
	patch := pb.Steps[0].Payload["patch"].(map[string]any)
	if patch["x"] != float64(220) || patch["y"] != float64(200) {
		t.Fatalf("patch=%#v", patch)
	}
}

func TestPcbLayoutPlanCommandRejectsCandidateCountAboveThree(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newPcbLayoutPlanCmd(&stdout, &stderr)
	cmd.SetArgs([]string{"--from", "a", "--board", "b", "--module", "m", "--out", "o", "--candidates", "4"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected local candidate limit error")
	}
}

func TestPcbLayoutPlanBundleAtomicallyReplacesOlderCandidateSet(t *testing.T) {
	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "out")
	snap := lpSnapshot(lpComp("p1", "U1", 200, 200, 0, lpBBox(180, 180, 220, 220)))
	in := pcbLayoutPlanInput{SchemaVersion: 1, Units: "mil", CoordinateSemantic: "footprint-anchor"}
	rep := pcbLayoutPlanReport{SchemaVersion: 1, Units: "mil", CoordinateSemantic: "footprint-anchor", Candidates: []pcbLayoutCandidate{{ID: "candidate-01"}, {ID: "candidate-02"}, {ID: "candidate-03"}}}
	if err := writePCBLayoutCandidateBundle(outDir, in, snap, &rep); err != nil {
		t.Fatal(err)
	}
	rep.Candidates = rep.Candidates[:1]
	if err := writePCBLayoutCandidateBundle(outDir, in, snap, &rep); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "candidate-02.apply.json")); !os.IsNotExist(err) {
		t.Fatalf("stale candidate survived replacement: %v", err)
	}
	for _, name := range []string{"manifest.json", "candidate-01.json", "candidate-01.apply.json", "candidate-01.svg"} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Fatalf("missing %s after replacement: %v", name, err)
		}
	}
}
