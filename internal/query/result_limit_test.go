package query

import "testing"

func TestExecuteWithLimitRejectsPartialResultSets(t *testing.T) {
	db := setup(t)
	results, err := ExecuteWithLimit(db, "people", nil, 2)
	if _, ok := err.(*ResultLimitError); !ok {
		t.Fatalf("err = %v, want ResultLimitError", err)
	}
	if len(results) != 0 {
		t.Fatalf("partial results = %#v, want none", results)
	}
}
