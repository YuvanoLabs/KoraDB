package query

import (
	"context"
	"errors"
	"testing"
)

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

func TestExecutePageContextHonorsCancelledRequest(t *testing.T) {
	db := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ExecutePageContext(ctx, db, "people", nil, 1, "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled page query = %v, want context.Canceled", err)
	}
}

func TestExecutePageTraversesBoundedPagesWithoutDuplicates(t *testing.T) {
	db := setup(t, "city")

	first, err := ExecutePage(db, "people", nil, 2, "")
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first.Results) != 2 || first.NextPageToken == "" {
		t.Fatalf("first page = %#v, want two results and a token", first)
	}
	second, err := ExecutePage(db, "people", nil, 2, first.NextPageToken)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(second.Results) != 2 || second.NextPageToken != "" {
		t.Fatalf("second page = %#v, want final two results", second)
	}

	seen := make(map[string]bool)
	for _, page := range []Page{first, second} {
		for _, result := range page.Results {
			if seen[result.ID] {
				t.Fatalf("duplicate document id %q across pages", result.ID)
			}
			seen[result.ID] = true
		}
	}
	if len(seen) != 4 {
		t.Fatalf("paged results = %d, want 4", len(seen))
	}
}

func TestExecutePageRejectsTokenForAnotherQuery(t *testing.T) {
	db := setup(t)
	page, err := ExecutePage(db, "people", Cmp{Field: "city", Op: Eq, Value: "NYC"}, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if page.NextPageToken == "" {
		t.Fatal("expected a continuation token")
	}
	_, err = ExecutePage(db, "people", Cmp{Field: "city", Op: Eq, Value: "LA"}, 1, page.NextPageToken)
	if !errors.Is(err, ErrInvalidPageToken) {
		t.Fatalf("token reuse error = %v, want ErrInvalidPageToken", err)
	}
}
