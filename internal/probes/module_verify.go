package probes

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type moduleVerification struct {
	Modules  []string
	Env      map[string]string
	Commands map[string]string
}

func verifyModules(modules []string) (moduleVerification, error) {
	cleanModules := cleanModuleList(modules)
	if len(cleanModules) == 0 {
		return moduleVerification{}, fmt.Errorf("no modules requested")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	args := append([]string{"-c", moduleVerifyScript, "cluster-inspector-module-verify"}, cleanModules...)
	cmd := exec.CommandContext(ctx, "bash", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return moduleVerification{}, fmt.Errorf("module verification timed out for %s", strings.Join(cleanModules, ","))
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return moduleVerification{}, fmt.Errorf("module verification failed for %s: %w: %s", strings.Join(cleanModules, ","), err, msg)
		}
		return moduleVerification{}, fmt.Errorf("module verification failed for %s: %w", strings.Join(cleanModules, ","), err)
	}
	verification := parseModuleVerification(stdout.String())
	verification.Modules = cleanModules
	return verification, nil
}

const moduleVerifyScript = `
set -u
if [ -r /etc/profile.d/modules.sh ]; then . /etc/profile.d/modules.sh >/dev/null 2>&1 || true; fi
if [ -r /usr/share/lmod/lmod/init/bash ]; then . /usr/share/lmod/lmod/init/bash >/dev/null 2>&1 || true; fi
if [ -r /usr/share/Modules/init/bash ]; then . /usr/share/Modules/init/bash >/dev/null 2>&1 || true; fi

if type module >/dev/null 2>&1; then
  module purge >/dev/null 2>&1 || module reset >/dev/null 2>&1 || true
  for module_name in "$@"; do
    module load "$module_name" >/dev/null 2>&1 || exit 43
  done
elif command -v modulecmd >/dev/null 2>&1; then
  eval "$(modulecmd bash purge 2>/dev/null || true)" >/dev/null 2>&1 || true
  for module_name in "$@"; do
    eval "$(modulecmd bash load "$module_name")" >/dev/null 2>&1 || exit 43
  done
else
  exit 42
fi

for key in CRAYPE_VERSION PE_ENV CRAY_PE_CCE_PREFIX CRAY_CC_VERSION GCC_PATH AOCC_HOME AOCC_ROOT AOMP INTEL_PATH INTEL_HOME ONEAPI_ROOT CMPLR_ROOT ROCM_PATH NVHPC_ROOT CRAY_MPICH_VERSION MPICH_DIR MPI_HOME I_MPI_ROOT CUDA_HOME CUDA_PATH; do
  value="${!key-}"
  printf 'ENV:%s=%s\n' "$key" "$value"
done

for command_name in cc CC ftn gcc g++ gfortran clang clang++ amdclang amdclang++ flang icx icpx ifx icc icpc ifort nvc nvc++ nvfortran mpicc mpirun hipcc nvcc; do
  command_path="$(command -v "$command_name" 2>/dev/null || true)"
  if [ -n "$command_path" ]; then
    printf 'CMD:%s=%s\n' "$command_name" "$command_path"
  fi
done
`

func parseModuleVerification(out string) moduleVerification {
	verification := moduleVerification{
		Env:      map[string]string{},
		Commands: map[string]string{},
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		kind, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key, value, ok := strings.Cut(rest, "=")
		if !ok {
			continue
		}
		switch kind {
		case "ENV":
			verification.Env[key] = value
		case "CMD":
			verification.Commands[key] = value
		}
	}
	return verification
}

func cleanModuleList(modules []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(modules))
	for _, module := range modules {
		module = strings.TrimSpace(module)
		if module == "" || seen[module] {
			continue
		}
		seen[module] = true
		out = append(out, module)
	}
	return out
}
