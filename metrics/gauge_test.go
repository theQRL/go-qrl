package metrics

import (
	"testing"
)

func BenchmarkGauge(b *testing.B) {
	g := NewGauge()
	for i := int64(0); b.Loop(); i++ {
		g.Update(i)
	}
}

func TestGaugeSnapshot(t *testing.T) {
	g := NewGauge()
	g.Update(int64(47))
	snapshot := g.Snapshot()
	g.Update(int64(0))
	if v := snapshot.Value(); v != 47 {
		t.Errorf("g.Value(): 47 != %v\n", v)
	}
}

func TestGetOrRegisterGauge(t *testing.T) {
	r := NewRegistry()
	NewRegisteredGauge("foo", r).Update(47)
	if g := GetOrRegisterGauge("foo", r); g.Snapshot().Value() != 47 {
		t.Fatal(g)
	}
}
