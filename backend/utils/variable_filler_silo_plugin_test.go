package utils

import (
	"aura/models"
	"strings"
	"testing"
)

func TestTemplateVarsSiloPlugin(t *testing.T) {
	item := models.MediaItem{Title: "Carrie"}
	set := models.DBPosterSetDetail{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "59977", UserCreated: "willtong93"}}}

	vars := TemplateVars_SiloPlugin(item, set, nil, nil)
	if vars["ReasonTitle"] != "Success" {
		t.Errorf("reason title = %q", vars["ReasonTitle"])
	}
	if !strings.Contains(vars["Reason"], `Silo used a set by "willtong93" from the`) || !strings.Contains(vars["Reason"], "Silo Plugin") || !strings.HasSuffix(vars["Reason"], "with no issues.") {
		t.Errorf("reason = %q", vars["Reason"])
	}
	if vars["SetID"] != "59977" || vars["MediaItemTitle"] != "Carrie" {
		t.Errorf("vars = %+v", vars)
	}

	vars = TemplateVars_SiloPlugin(item, set, []string{"Poster: failed"}, []string{"Backdrop: slow"})
	if vars["ReasonTitle"] != "Error" || !strings.Contains(vars["Reason"], "Poster: failed") || !strings.Contains(vars["Reason"], "Backdrop: slow") {
		t.Errorf("errors = %q %q", vars["ReasonTitle"], vars["Reason"])
	}
}
