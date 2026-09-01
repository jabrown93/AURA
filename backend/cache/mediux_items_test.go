package cache

import (
	"aura/models"
	"testing"
)

func TestStoreMediuxItemsReplacesSnapshotWithoutDuplicates(t *testing.T) {
	items := NewMediuxItemCache()
	items.StoreMediuxItems(
		[]models.MediuxContentID{{ID: "1"}, {ID: "1"}, {ID: "2"}, {ID: ""}},
		[]models.MediuxContentID{{ID: "10"}, {ID: "10"}},
	)

	if movies, shows := items.GetCountMediuxItems(); movies != 2 || shows != 1 {
		t.Fatalf("initial counts = (%d, %d), want (2, 1)", movies, shows)
	}

	items.AddItem("movie", "local")
	items.RemoveItem("show", "10")
	items.StoreMediuxItems(
		[]models.MediuxContentID{{ID: "2"}, {ID: "3"}, {ID: "3"}},
		[]models.MediuxContentID{{ID: "20"}},
	)

	if movies, shows := items.GetCountMediuxItems(); movies != 2 || shows != 1 {
		t.Fatalf("replacement counts = (%d, %d), want (2, 1)", movies, shows)
	}
	for _, absent := range []struct {
		itemType string
		id       string
	}{{"movie", "1"}, {"movie", "local"}, {"show", "10"}} {
		if items.CheckItemExists(absent.itemType, absent.id) {
			t.Errorf("stale item %s/%s remained after snapshot replacement", absent.itemType, absent.id)
		}
	}
	for _, present := range []struct {
		itemType string
		id       string
	}{{"movie", "2"}, {"movie", "3"}, {"show", "20"}} {
		if !items.CheckItemExists(present.itemType, present.id) {
			t.Errorf("snapshot item %s/%s missing after replacement", present.itemType, present.id)
		}
	}
}
