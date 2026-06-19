package probes

import "errors"

// ProbeNode runs the per-node-type probes: CPU target detection, GPU
// presence + facts (delegates to ProbeGPU), and build-stage candidate
// discovery (writability, free space, mount options, throughput class).
//
// TODO: Phase 3 — read /proc/cpuinfo + lscpu, run ProbeGPU, scan
// candidate build-stage paths from $TMPDIR / /local_scratch / /scratch /
// scheduler vars / hints. Test writability with a tiny create/remove
// probe (cleanup before return). See extraction map § Section 2 + § GPU
// Node Facts.
func ProbeNode() error {
	return errors.New("probes.ProbeNode: not yet implemented (Phase 3)")
}
