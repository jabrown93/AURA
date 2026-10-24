package downloadqueue

import (
	"aura/models"
	"testing"
)

func TestRetryableFailureSets(t *testing.T) {
	if got := retryableFailureSets(models.DBSavedItem{}); len(got) != 1 || got[0].ID != "" {
		t.Errorf("no sets: got %+v, want one set-less failure", got)
	}
	item := models.DBSavedItem{PosterSets: []models.DBPosterSetDetail{{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "a"}}}, {PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "b"}}}}}
	got := retryableFailureSets(item)
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Errorf("got %+v, want sets a and b", got)
	}
}
