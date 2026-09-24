package paginator

import (
	"errors"
	"iter"
)

// A simple paginator for arbitruary slices, pages are zero-indexed for simplicity
type Paginator[T ~[]E, E any] struct {
	data       T
	size       int
	pageNumber int
	pageSize   int
}

var errOutOfBoundsPage = errors.New("page is out of bounds")

func NewPaginator[T []E, E any](data T, pageSize int, pageNumber int) (*Paginator[T, E], error) {
	size := len(data)

	if pageSize*pageNumber >= size {
		return nil, errOutOfBoundsPage
	}

	return &Paginator[T, E]{
		data:       data,
		size:       size,
		pageNumber: pageNumber,
		pageSize:   pageSize,
	}, nil
}

func (p *Paginator[T, E]) pageBoundaries() (start, end int) {
	start = p.pageNumber * p.pageSize

	// The last page is short whenever the data does not divide evenly.
	end = min(start+p.pageSize, p.size)

	return
}

func (p *Paginator[T, E]) Size() int {
	return p.size
}

// Returns the paginated slice
func (p *Paginator[T, E]) Slice() T {
	start, end := p.pageBoundaries()
	return p.data[start:end]
}

func (p *Paginator[T, E]) IterCurrentPage() iter.Seq2[int, E] {
	return func(yield func(int, E) bool) {
		start, end := p.pageBoundaries()
		for index, item := range p.data[start:end] {
			if !yield(start+index, item) {
				break
			}
		}
	}
}

var errCantSetPage = errors.New("can't set page")

func (p *Paginator[T, E]) SetPage(pageNumber int) error {
	// Mirrors the bound NewPaginator rejects on: the first item of the page has
	// to exist, otherwise the page is empty.
	if pageNumber*p.pageSize >= p.size {
		return errCantSetPage
	}
	p.pageNumber = pageNumber
	return nil
}

func (p *Paginator[T, E]) PageNumber() int {
	return p.pageNumber
}

func (p *Paginator[T, E]) PageSize() int {
	return p.pageSize
}

var errNoNext = errors.New("no next page")

// Moves current paginator to the next page
func (p *Paginator[T, E]) Next() error {
	if !p.HasNext() {
		return errNoNext
	}
	p.pageNumber = p.pageNumber + 1
	return nil
}

func (p *Paginator[T, E]) HasNext() bool {
	return (p.pageNumber+1)*p.pageSize < p.size
}

var errNoPrev = errors.New("no prev page")

// Moves current paginator to the previous page
func (p *Paginator[T, E]) Prev() error {
	if !p.HasPrev() {
		return errNoPrev
	}
	p.pageNumber = p.pageNumber - 1
	return nil
}

func (p *Paginator[T, E]) HasPrev() bool {
	return p.pageNumber > 0
}
