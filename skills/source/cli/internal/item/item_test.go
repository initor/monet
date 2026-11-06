package item

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"source/internal/store"
)

func TestItemSaveLoad(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	it, w, it2, w2 := Item{"D1", "spec", "p1", 2, []string{"F1"}, "ruled", "keep", "revert", "spec", "other", "w3"}, Wall{"W1", "dev", "q", "d", "b", "cleared", "w4"}, Item{}, Wall{}
	errs := []error{Save(d, "D1", it), Save(d, "W1", w), Load(d, "D1", &it2), Load(d, "W1", &w2)}
	if b, _ := json.Marshal([]any{it, Item{}, w, Wall{}}); !reflect.DeepEqual(it2, it) || !reflect.DeepEqual(w2, w) || !reflect.DeepEqual(errs, make([]error, 4)) || Path(d, "W1") != d.Root+"/items/W1.json" || string(b) != `[{"id":"D1","phase":"spec","persona":"p1","round":2,"findings":["F1"],"status":"ruled","default":"keep","way_back":"revert","affects":"spec","pick":"other","words":"w3"},{"id":"","phase":"","persona":"","round":0,"findings":null,"status":""},{"id":"W1","by":"dev","question":"q","default":"d","blocks":"b","status":"cleared","words":"w4"},{"id":"","by":"","question":"","default":"","blocks":"","status":""}]` {
		t.Fatalf("%+v %+v %v %s", it2, w2, errs, b)
	}
}

func TestWaiting(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	os.MkdirAll(d.Path("items"), 0o755)
	got, errs := [][]string{}, []error{}
	for _, stage := range []map[string]any{{}, {"D1": Item{Status: "ruled"}, "D2": Item{Status: "open"}, "D10": Item{Status: "draft"}, "W1": Wall{Status: "cleared"}, "W2": Wall{Status: "open"}}, {"D3": Item{Status: "open"}, "W10": Wall{Status: "open"}}} {
		for id, v := range stage {
			Save(d, id, v)
		}
		items, walls, err := Waiting(d)
		got, errs = append(got, items, walls), append(errs, err)
	}
	os.WriteFile(Path(d, "D4"), []byte(`{"x":1}`), 0o644)
	if _, _, bad := Waiting(d); !reflect.DeepEqual(got, [][]string{nil, nil, {"D2", "D10"}, {"W2"}, {"D2", "D3", "D10"}, {"W2", "W10"}}) || !reflect.DeepEqual(errs, make([]error, 3)) || bad == nil {
		t.Fatalf("waiting %v %v, bad item %v", got, errs, bad)
	}
}
