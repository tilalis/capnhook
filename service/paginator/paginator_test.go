package paginator

import (
	"slices"
	"testing"
)

func TestNewPaginator(t *testing.T) {
	_, err := NewPaginator([]int{1, 2, 3, 4}, 4, 1)
	if err == nil {
		t.Errorf("expected error %v, got %v", errOutOfBoundsPage, err)
	}
	_, err = NewPaginator([]int{1, 2, 3, 4}, 4, 0)
	if err != nil {
		t.Errorf("expected error %v, got %v", errOutOfBoundsPage, err)
	}
}

func TestPaginatorSlice(t *testing.T) {
	tests := []struct {
		name          string
		data          []int
		pageSize      int
		expectedPages [][]int
	}{
		{
			name:     "evenly divided",
			data:     []int{0, 1, 2, 3, 4, 5},
			pageSize: 3,
			expectedPages: [][]int{
				{0, 1, 2},
				{3, 4, 5},
			},
		},
		{
			name:     "paged with leftovers",
			data:     []int{0, 1, 2, 3, 4},
			pageSize: 2,
			expectedPages: [][]int{
				{0, 1},
				{2, 3},
				{4},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paginator, _ := NewPaginator(test.data, test.pageSize, 0)

			lastPage := len(test.expectedPages) - 1
			for i, expectedPage := range test.expectedPages {
				got := paginator.Slice()
				if !slices.Equal(got, expectedPage) {
					t.Errorf("on page %d: got %v, want %v", i, got, expectedPage)
				}

				hasNext := paginator.HasNext()
				expectedHasNext := i != lastPage

				if hasNext != expectedHasNext {
					t.Errorf(
						"on page %d: expected hasNext to be %t, got %t",
						i,
						expectedHasNext,
						hasNext,
					)
				}

				if err := paginator.Next(); (hasNext && err != nil) || (!hasNext && err != errNoNext) {
					t.Errorf(
						"on page %d: hasNext is %t, but %v",
						i,
						hasNext,
						err,
					)
				}
			}

			hasPrev := paginator.HasPrev()

			if !hasPrev {
				t.Errorf(
					"hasPrev is %t, expected true",
					hasPrev,
				)
			}

			if err := paginator.Prev(); err != nil {
				t.Errorf("error when calling prev %v", err)
			}

			expectedPage := test.expectedPages[len(test.expectedPages)-2]

			if got := paginator.Slice(); !slices.Equal(got, expectedPage) {
				t.Errorf("on previous page got %v, want %v", got, expectedPage)
			}
		})
	}
}
