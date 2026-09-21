package app

import (
	"math"
	"testing"
)

func TestRectViaFencePointsKeepsCornersUniqueAndPitchBounded(t *testing.T) {
	points, err := rectViaFencePoints(0, 0, 100, 60, 40, 10)
	if err != nil {
		t.Fatal(err)
	}
	// Expanded rectangle is -10,-10 .. 110,70.  The horizontal edges use
	// 3 intervals and the vertical edges use 2: 2*(3+2) = 10 unique points.
	if len(points) != 10 {
		t.Fatalf("got %d points, want 10: %#v", len(points), points)
	}
	seen := map[[2]float64]bool{}
	for _, point := range points {
		if seen[point] {
			t.Fatalf("duplicate corner/point: %#v", point)
		}
		seen[point] = true
	}
	for _, corner := range [][2]float64{{-10, -10}, {110, -10}, {110, 70}, {-10, 70}} {
		if !seen[corner] {
			t.Fatalf("missing corner %#v in %#v", corner, points)
		}
	}
	for i := range points {
		next := points[(i+1)%len(points)]
		if distance := math.Hypot(next[0]-points[i][0], next[1]-points[i][1]); distance > 40+1e-9 {
			t.Fatalf("edge spacing %.6f exceeds pitch: %#v -> %#v", distance, points[i], next)
		}
	}
}

func TestRectViaFencePointsNormalizesRectAndRejectsInvalidInputs(t *testing.T) {
	forward, err := rectViaFencePoints(0, 0, 100, 60, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	reversed, err := rectViaFencePoints(100, 60, 0, 0, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(forward) != len(reversed) {
		t.Fatalf("normalization changed count: %d != %d", len(forward), len(reversed))
	}
	for i := range forward {
		if forward[i] != reversed[i] {
			t.Fatalf("normalization changed point %d: %#v != %#v", i, forward[i], reversed[i])
		}
	}
	if _, err := rectViaFencePoints(0, 0, 0, 60, 40, 0); err == nil {
		t.Fatal("zero-width rectangle should fail")
	}
	if _, err := rectViaFencePoints(0, 0, 100, 60, 0, 0); err == nil {
		t.Fatal("zero pitch should fail")
	}
	if _, err := rectViaFencePoints(0, 0, 100, 60, 40, -1); err == nil {
		t.Fatal("negative margin should fail")
	}
}
