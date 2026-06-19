package probes

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ray12514/cluster-inspector/internal/model"
)

var versionPattern = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)+`)

func evidence(confidence model.Confidence, source string) model.Evidence {
	return model.Evidence{Confidence: confidence, Source: source}
}

func run(name string, args ...string) (string, error) {
	ctx := exec.Command(name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &stderr
	if err := ctx.Start(); err != nil {
		return "", err
	}

	done := make(chan error, 1)
	go func() {
		done <- ctx.Wait()
	}()

	select {
	case err := <-done:
		out := strings.TrimSpace(stdout.String())
		if out == "" {
			out = strings.TrimSpace(stderr.String())
		}
		if err != nil {
			return out, err
		}
		return out, nil
	case <-time.After(3 * time.Second):
		_ = ctx.Process.Kill()
		return "", errors.New("command timed out")
	}
}

func commandPath(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

func firstVersion(s string) string {
	return versionPattern.FindString(s)
}

func splitVersion(version string) (int, *int) {
	major := 0
	minor := (*int)(nil)
	fields := regexp.MustCompile(`[0-9]+`).FindAllString(version, 2)
	if len(fields) > 0 {
		major, _ = strconv.Atoi(fields[0])
	}
	if len(fields) > 1 {
		value, _ := strconv.Atoi(fields[1])
		minor = &value
	}
	return major, minor
}

func prefixFromCommand(path string) string {
	if path == "" || !filepath.IsAbs(path) {
		return ""
	}
	dir := filepath.Dir(path)
	if filepath.Base(dir) == "bin" || filepath.Base(dir) == "sbin" {
		return filepath.Dir(dir)
	}
	return dir
}

func envList(name string) []string {
	value := os.Getenv(name)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ":")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func firstExistingDir(paths ...string) string {
	for _, path := range paths {
		if isDir(path) {
			return path
		}
	}
	return ""
}

func appendEvidence(dst map[string]model.Evidence, key string, value model.Evidence) {
	if dst == nil {
		return
	}
	dst[key] = value
}

func commandSource(name string, args ...string) string {
	if len(args) == 0 {
		return name
	}
	return fmt.Sprintf("%s %s", name, strings.Join(args, " "))
}
