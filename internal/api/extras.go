package api

import "strconv"

// extras holds state that belongs to single APIs rather than to the server as a whole:
// things loaded on first use and dropped when a dataset changes.
type extras struct{}

// datasetChanged drops whatever was derived from a dataset that was just replaced.
func (e *extras) datasetChanged(name string) {}

func (e *extras) close() {}

// registerMore registers the routes of the APIs added after the first release. Each
// lives in its own file.
func (s *Server) registerMore() {}

func itoa(value int) string { return strconv.Itoa(value) }
