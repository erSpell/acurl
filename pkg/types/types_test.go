package types

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDurationSerialisesAsMilliseconds(t *testing.T) {
	var r FetchResult
	r.SetDuration(1500 * time.Millisecond)
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"duration_ms":1500`) {
		t.Errorf("got %s, want duration_ms 1500", b)
	}
}
