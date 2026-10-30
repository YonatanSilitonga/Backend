package admin

import "testing"

func TestNormalisasiImplantTitik(t *testing.T) {
	t.Run("default self", func(t *testing.T) {
		got, err := normalisasiImplantTitik([]int64{1, 2}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got[1] != 1 || got[2] != 2 {
			t.Fatalf("expected self mapping, got %#v", got)
		}
	})

	t.Run("group to host", func(t *testing.T) {
		got, err := normalisasiImplantTitik([]int64{1, 2, 3}, map[int64]int64{1: 1, 2: 1, 3: 1})
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []int64{1, 2, 3} {
			if got[id] != 1 {
				t.Fatalf("implant %d expected host 1, got %#v", id, got)
			}
		}
	})

	t.Run("host outside selection", func(t *testing.T) {
		if _, err := normalisasiImplantTitik([]int64{1, 2}, map[int64]int64{1: 1, 2: 9}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("host must point to itself", func(t *testing.T) {
		if _, err := normalisasiImplantTitik([]int64{1, 2}, map[int64]int64{1: 2, 2: 1}); err == nil {
			t.Fatal("expected error")
		}
	})
}
