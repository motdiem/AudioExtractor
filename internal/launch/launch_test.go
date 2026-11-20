package launch

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestFilter(t *testing.T) {
	exists := func(p string) bool { return p != `C:\gone.mp4` }
	got := Filter([]string{"-psn_0_123", `C:\a.mp4`, "", `C:\gone.mp4`, "--flag", "  ", `C:\Zoom folder`}, exists)
	want := []string{`C:\a.mp4`, `C:\Zoom folder`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFilterDefaultChecksDisk(t *testing.T) {
	dir := t.TempDir()
	got := Filter([]string{dir, dir + "/nope"}, nil)
	if !reflect.DeepEqual(got, []string{dir}) {
		t.Fatalf("got %q", got)
	}
}

type sink struct {
	mu      sync.Mutex
	batches [][]string
}

func (s *sink) add(b []string) { s.mu.Lock(); s.batches = append(s.batches, b); s.mu.Unlock() }
func (s *sink) get() [][]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]string(nil), s.batches...)
}

func TestCoalescerMergesQuickAdds(t *testing.T) {
	var s sink
	c := NewCoalescer(60*time.Millisecond, s.add)
	c.Add("a")
	time.Sleep(10 * time.Millisecond)
	c.Add("b", "c")
	time.Sleep(10 * time.Millisecond)
	c.Add("d")
	time.Sleep(250 * time.Millisecond)
	want := [][]string{{"a", "b", "c", "d"}}
	if got := s.get(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCoalescerSeparatesSlowAdds(t *testing.T) {
	var s sink
	c := NewCoalescer(20*time.Millisecond, s.add)
	c.Add("a")
	time.Sleep(150 * time.Millisecond)
	c.Add("b")
	time.Sleep(150 * time.Millisecond)
	if got := s.get(); len(got) != 2 {
		t.Fatalf("want 2 batches, got %q", got)
	}
}

func TestCoalescerIgnoresEmpty(t *testing.T) {
	var s sink
	c := NewCoalescer(10*time.Millisecond, s.add)
	c.Add()
	time.Sleep(60 * time.Millisecond)
	if got := s.get(); len(got) != 0 {
		t.Fatalf("got %q", got)
	}
}

func TestLauncherHoldsUntilReady(t *testing.T) {
	var s sink
	l := New(10*time.Millisecond, s.add)
	l.Add("a")
	time.Sleep(80 * time.Millisecond)
	l.Add("b")
	time.Sleep(80 * time.Millisecond)
	if got := s.get(); len(got) != 0 {
		t.Fatalf("delivered before ready: %q", got)
	}
	l.Ready()
	if got := s.get(); !reflect.DeepEqual(got, [][]string{{"a", "b"}}) {
		t.Fatalf("got %q", got)
	}
	l.Add("c")
	time.Sleep(80 * time.Millisecond)
	if got := s.get(); len(got) != 2 || got[1][0] != "c" {
		t.Fatalf("got %q", got)
	}
}
