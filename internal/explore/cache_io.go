package explore

import "strings"

// CacheIOError is recoverable only after snapshot validation. Validation, encoding,
// size-limit and cancellation errors are never converted into cache warnings.
type CacheIOError struct {
	Stage string
	Err   error
}

func (e *CacheIOError) Error() string {
	return "CACHE_IO_" + strings.ToUpper(strings.ReplaceAll(e.Stage, "-", "_"))
}
func (e *CacheIOError) Unwrap() error { return e.Err }
func (c Cache) io(stage string, run func() error) error {
	if c.failIO != nil {
		if err := c.failIO(stage); err != nil {
			return &CacheIOError{stage, err}
		}
	}
	if err := run(); err != nil {
		return &CacheIOError{stage, err}
	}
	return nil
}
