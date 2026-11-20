package extract

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/motdiem/AudioExtractor/internal/i18n"
)

// waitIdle waits for the job started in the background to finish.
func waitIdle(t *testing.T, svc *Service) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if svc.busy.TryLock() {
			svc.busy.Unlock()
			return
		}
	}
	t.Fatal("the job never finished")
}

// openPicker starts Pick with the picker left open, and returns once it is.
func openPicker(t *testing.T, svc *Service, port *fakePort) (pickDone chan Result) {
	t.Helper()
	port.pickGate, port.pickOpen = make(chan struct{}), make(chan struct{}, 1)
	pickDone = make(chan Result, 1)
	go func() { pickDone <- svc.Pick() }()
	select {
	case <-port.pickOpen:
	case <-time.After(5 * time.Second):
		t.Fatal("the picker never opened")
	}
	return pickDone
}

// While the picker is open, files arriving (a launch, a drop) are refused
// with the busy message, never silently, and only one job ever runs.
func TestTryRunIsRefusedWithTheBusyMessageWhileThePickerIsOpen(t *testing.T) {
	port := &fakePort{}
	svc := New(Config{Port: port, Lang: i18n.EN, Locate: func() (string, error) { return "ffmpeg", nil }})
	pickDone := openPicker(t, svc, port)

	var wg sync.WaitGroup
	started := make(chan bool, 10)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			started <- svc.TryRun([]string{"/nonexistent.mp4"}, "")
		}()
	}
	wg.Wait()
	close(started)
	for ok := range started {
		if ok {
			t.Error("a batch started while the picker was open")
		}
	}
	if res := svc.Run([]string{"/nonexistent.mp4"}, ""); !res.Cancelled {
		t.Errorf("Run while busy = %+v, want refused", res)
	}
	if n := port.infoCount(); n != 11 {
		t.Errorf("busy message shown %d times, want 11 (once per refusal)", n)
	}
	if port.infos[0] != "AudioExtractor is busy" {
		t.Errorf("info title = %q", port.infos[0])
	}

	close(port.pickGate) // the user cancels the picker
	if res := <-pickDone; !res.Cancelled {
		t.Errorf("Pick = %+v, want cancelled", res)
	}
	if !svc.TryRun([]string{"/nonexistent.mp4"}, "") {
		t.Error("TryRun refused once the service was free")
	}
	waitIdle(t, svc)
}

// A Pick while a batch runs is refused out loud too.
func TestPickWhileBusyShowsTheBusyMessage(t *testing.T) {
	port := &fakePort{}
	svc := New(Config{Port: port, Lang: i18n.FR})
	pickDone := openPicker(t, svc, port)
	if res := svc.Pick(); !res.Cancelled {
		t.Errorf("second Pick = %+v, want refused", res)
	}
	if port.infoCount() != 1 || port.infos[0] != "AudioExtractor est occupé" {
		t.Errorf("infos = %q, want the French busy title once", port.infos)
	}
	close(port.pickGate)
	<-pickDone
}

// Quitting mid-extraction kills ffmpeg and removes the .part file before the
// app goes, and nothing new starts afterwards.
func TestShutdownCancelsTheJobAndCleansUp(t *testing.T) {
	src := makeAudioWith(t, "huge.wav", 1200, 2, "pcm_s16le", "16000") // long enough to still be encoding
	dir := t.TempDir()
	port := &fakePort{saveTo: filepath.Join(dir, "huge.mp3")}
	working := make(chan struct{})
	var once sync.Once
	bin := ffmpegPath(t)
	svc := New(Config{
		Locate: func() (string, error) { return bin, nil },
		Port:   port,
		Lang:   i18n.EN,
		Emit: func(_ string, data any) {
			if e := data.(StateEvent); e.Phase == PhaseWorking && e.Percent >= 0 {
				once.Do(func() { close(working) })
			}
		},
	})
	if !svc.TryRun([]string{src}, "mp3") {
		t.Fatal("TryRun refused on an idle service")
	}
	select {
	case <-working:
	case <-time.After(20 * time.Second):
		t.Fatal("the encode never started")
	}

	start := time.Now()
	if err := svc.ServiceShutdown(); err != nil {
		t.Fatalf("ServiceShutdown: %v", err)
	}
	if d := time.Since(start); d > shutdownWait {
		t.Errorf("shutdown took %v", d)
	}
	noStrays(t, dir) // neither the output nor the .part file

	if svc.TryRun([]string{src}, "mp3") {
		t.Error("a job started after shutdown")
	}
	if port.infoCount() != 0 {
		t.Error("the busy message was shown while quitting")
	}
}

func TestShutdownWhenIdleReturnsAtOnce(t *testing.T) {
	svc := New(Config{Port: &fakePort{}, Lang: i18n.EN})
	start := time.Now()
	if err := svc.ServiceShutdown(); err != nil || time.Since(start) > 100*time.Millisecond {
		t.Errorf("err = %v after %v", err, time.Since(start))
	}
}
