package hints

import "errors"

// Apply narrows a candidate list to those that pass an `include` filter
// (when non-empty), then drops anything matching `exclude_patterns`, then
// appends `extras`. Used per category (compilers, mpi, gpu_toolkits,
// fabric_userspace).
//
// TODO: Phase 4 — implement include/exclude/extras semantics per design
// doc § Module Discovery And Hints.
func Apply() error {
	return errors.New("hints.Apply: not yet implemented (Phase 4)")
}
