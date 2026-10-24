package mediaserver

import "testing"

func TestLibraryProgress(t *testing.T) {
	ch, _ := SubscribeLibraryProgress()
	defer UnsubscribeLibraryProgress(ch)

	startLibraryProgress([]string{"Movies", "Series"}, map[string]int{"Series": 500})
	updateLibraryLoad("Movies", 1000, 2500, false)
	// Only the newest progress is waiting
	if p := <-ch; !p.Running || p.Libraries[0].Loaded != 1000 || p.Libraries[0].Total != 2500 || p.Libraries[1].Loaded != 0 || p.Libraries[1].Total != 500 {
		t.Errorf("progress = %+v", p)
	}

	updateLibraryLoad("Movies", 2500, 2500, true)
	finishLibraryProgress(true)
	if p := <-ch; p.Running || p.Failed || !p.Libraries[0].Done || p.FinishedAt == 0 {
		t.Errorf("finished progress = %+v", p)
	}
	if p := CurrentLibraryProgress(); p.Running || len(p.Libraries) != 2 {
		t.Errorf("current progress = %+v", p)
	}
}
