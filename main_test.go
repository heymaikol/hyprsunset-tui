package main

import "testing"

func TestClamp(t *testing.T) {
	if clamp(500, tempMin, tempMax) != tempMin {
		t.Fatal("below min not clamped")
	}
	if clamp(99999, tempMin, tempMax) != tempMax {
		t.Fatal("above max not clamped")
	}
	if clamp(4000, tempMin, tempMax) != 4000 {
		t.Fatal("in-range changed")
	}
}

func TestPresetKeysMapToIndex(t *testing.T) {
	for i, key := range []byte{'1', '2', '3'} {
		if int(key-'1') != i {
			t.Fatalf("preset key %c maps to wrong index", key)
		}
	}
	if presets[0].name != "Day" || presets[2].temp != 3000 {
		t.Fatal("preset table wrong")
	}
}
