package conformance

import "testing"

// TestGoVMMatchesVectors runs every entry in Vectors through this package's
// Go VM and asserts it matches the vector's own expectations. This is the
// self-consistency check; TestGoVMMatchesQrvmone (cross_test.go) compares
// Run() output with a matching qrvmone (C++) run.
func TestGoVMMatchesVectors(t *testing.T) {
	for _, v := range Vectors {
		t.Run(v.Name, func(t *testing.T) {
			got, err := Run(v)
			if err != nil {
				t.Fatalf("runner error: %v", err)
			}
			if diff := Diff(v, got); diff != "" {
				t.Errorf("mismatch: %s\n  got %+v\n  vector %+v", diff, got, v)
			}
		})
	}
}
