package utils

import (
	"aura/models"
	"strings"
	"testing"
)

func TestTemplateVarsNewItemAutoDownload(t *testing.T) {
	item := models.MediaItem{Title: "Animals"}
	set := models.DBPosterSetDetail{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "123", UserCreated: "willtong93"}}}

	vars := TemplateVars_NewItemAutoDownload(item, set, nil, nil, true)
	want := `A new media item was detected and aura found a set by one of your favorite creators "willtong93". This set was automatically downloaded with no issues.`
	if vars["ReasonTitle"] != "Success" || !strings.HasSuffix(vars["Reason"], want[len("A new media item was detected and aura"):]) || !strings.HasPrefix(vars["Reason"], "A new media item was detected") {
		t.Errorf("reason = %q, %q", vars["ReasonTitle"], vars["Reason"])
	}
	if vars["SetID"] != "123" || vars["SetCreator"] != "willtong93" || vars["MediaItemTitle"] != "Animals" {
		t.Errorf("vars = %+v", vars)
	}

	// Run for All Items, with warnings
	vars = TemplateVars_NewItemAutoDownload(item, set, nil, []string{"Poster: not found"}, false)
	if vars["ReasonTitle"] != "Warning" || !strings.HasPrefix(vars["Reason"], "This item had no saved set") || !strings.Contains(vars["Reason"], "Poster: not found") {
		t.Errorf("reason = %q, %q", vars["ReasonTitle"], vars["Reason"])
	}
}
