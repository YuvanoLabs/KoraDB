package query

import "testing"

func TestAutoKeyQueryIDsRoundTrip(t *testing.T) {
	db := setup(t)
	results, err := Execute(db, "people", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if _, err := db.Get("people", result.ID); err != nil {
			t.Fatalf("query id %q did not round-trip through Get: %v", result.ID, err)
		}
	}
}
