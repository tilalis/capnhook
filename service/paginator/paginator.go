package paginator

import (
	"errors"
)

// A simple paginator for arbitruary slices, pages are zero-indexed for simplicity
type Paginator[T []E, E any] struct {
	data       T
	size       int
	pageNumber int
	pageSize   int
}

var errOutOfBoundsPage = errors.New("page is out of bounds")

func NewPaginator[T []E, E any](data T, pageSize int, pageNumber int) (*Paginator[T, E], error) {
	size := len(data)

	if pageSize * pageNumber >= size {
		return nil, errOutOfBoundsPage
	}

	return &Paginator[T, E]{
		data:       data,
		size:       size,
		pageNumber: pageNumber,
		pageSize:   pageSize,
	}, nil
}

// Returns the paginated slice
func (p *Paginator[T, E]) Slice() T {
	start := p.pageNumber * p.pageSize
	end := start + p.pageSize

	if end > len(p.data) {
		return p.data[start:]
	}

	return p.data[start:end]
}

func (p Paginator[T, E]) PageNumber() int {
	return p.pageNumber
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
