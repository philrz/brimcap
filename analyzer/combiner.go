package analyzer

import (
	"context"
	"slices"
	"sync"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/zio"
)

// A Combiner is a zio.Reader that returns records by reading from multiple Readers.
type Combiner struct {
	arena   *zed.Arena // Owns the zed.Value returned by Read
	cancel  context.CancelFunc
	ctx     context.Context
	done    []bool
	once    sync.Once
	readers []zio.Reader
	results chan combinerResult
}

func NewCombiner(ctx context.Context, readers []zio.Reader) *Combiner {
	ctx, cancel := context.WithCancel(ctx)
	return &Combiner{
		cancel:  cancel,
		ctx:     ctx,
		done:    make([]bool, len(readers)),
		readers: readers,
		results: make(chan combinerResult),
	}
}

type combinerResult struct {
	err   error
	idx   int
	val   *zed.Value
	arena *zed.Arena
}

func (c *Combiner) run() {
	for i := range c.readers {
		idx := i
		go func() {
			for {
				rec, err := c.readers[idx].Read()
				var arena *zed.Arena
				if rec != nil {
					arena = zed.NewArena()
					// Make a copy since we don't wait for
					// Combiner.Read's caller to finish with
					// this value before we read the next.
					rec = rec.Copy(arena).Ptr()
				}
				select {
				case c.results <- combinerResult{err, idx, rec, arena}:
					if rec == nil || err != nil {
						return
					}
				case <-c.ctx.Done():
					return
				}
			}
		}()
	}
}

func (c *Combiner) finished() bool {
	return !slices.Contains(c.done, false)
}

func (c *Combiner) Read() (*zed.Value, error) {
	c.once.Do(c.run)
	for {
		select {
		case r := <-c.results:
			if r.err != nil {
				c.cancel()
				return nil, r.err
			}
			if r.val != nil {
				if c.arena != nil {
					c.arena.Unref()
				}
				c.arena = r.arena
				return r.val, nil
			}
			c.done[r.idx] = true
			if c.finished() {
				c.cancel()
				return nil, nil
			}
		case <-c.ctx.Done():
			return nil, c.ctx.Err()
		}
	}
}
