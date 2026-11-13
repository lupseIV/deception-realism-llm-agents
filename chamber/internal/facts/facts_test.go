package facts

import "testing"

func TestMergeDoesNotModifyInputs(t *testing.T) {
	a := Facts{"x": "1", "y": "1"}
	b := Facts{"y": "2", "z": "2"}
	m := a.Merge(b)
	if m["x"] != "1" || m["y"] != "2" || m["z"] != "2" {
		t.Fatalf("bad merge %v", m)
	}
	if a["y"] != "1" || len(a) != 2 || len(b) != 2 {
		t.Fatal("inputs modified")
	}
}

func TestKeysSorted(t *testing.T) {
	ks := Facts{"b": "", "a": "", "c": ""}.Keys()
	if ks[0] != "a" || ks[1] != "b" || ks[2] != "c" {
		t.Fatalf("not sorted: %v", ks)
	}
}
