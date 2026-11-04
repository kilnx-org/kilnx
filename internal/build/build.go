package build

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Bundle is everything a standalone binary needs from the project: the
// resolved source with imports inlined, and the raw text of each static
// custom-field manifest keyed by model name.
type Bundle struct {
	Source    string            `json:"source"`
	Manifests map[string]string `json:"manifests,omitempty"`
}

// magic marks the end of a binary that carries a bundle. The layout is
// <executable><bundle JSON><bundle length, 8 bytes LE><magic>.
const magic = "KILNX-BUNDLE-V01"

const trailerSize = 8 + len(magic)

// Build writes a standalone binary to outputPath: a copy of the running
// kilnx executable with the bundle appended. It needs neither the Go
// toolchain nor the kilnx source tree, and targets the host OS and
// architecture. It returns the path written.
func Build(bundle Bundle, kilnxFile, outputPath string) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locating kilnx executable: %w", err)
	}
	return write(self, bundle, kilnxFile, outputPath)
}

// Read returns the bundle appended to the running executable, or nil for a
// plain kilnx CLI.
func Read() (*Bundle, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locating executable: %w", err)
	}
	return read(self)
}

func write(exePath string, bundle Bundle, kilnxFile, outputPath string) (string, error) {
	if outputPath == "" {
		base := filepath.Base(kilnxFile)
		outputPath = strings.TrimSuffix(base, filepath.Ext(base))
	}
	if runtime.GOOS == "windows" && filepath.Ext(outputPath) == "" {
		outputPath += ".exe"
	}
	absOutput, err := filepath.Abs(outputPath)
	if err != nil {
		return "", fmt.Errorf("resolving path %s: %w", outputPath, err)
	}
	absSource, err := filepath.Abs(kilnxFile)
	if err != nil {
		return "", fmt.Errorf("resolving path %s: %w", kilnxFile, err)
	}
	if absOutput == absSource {
		return "", fmt.Errorf("output path %s would overwrite the source file", outputPath)
	}
	if same, err := sameFile(absOutput, exePath); err != nil {
		return "", err
	} else if same {
		return "", fmt.Errorf("output path %s would overwrite the kilnx executable", outputPath)
	}

	payload, err := json.Marshal(bundle)
	if err != nil {
		return "", fmt.Errorf("encoding bundle: %w", err)
	}

	exe, err := os.Open(exePath) //nolint:gosec // exePath is os.Executable, not user input
	if err != nil {
		return "", fmt.Errorf("opening kilnx executable: %w", err)
	}
	defer func() { _ = exe.Close() }()

	// Write next to the target and rename, so a failed build never leaves a
	// truncated binary and a running one can be replaced in place.
	tmp, err := os.CreateTemp(filepath.Dir(absOutput), ".kilnx-build-*")
	if err != nil {
		return "", fmt.Errorf("creating output: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // already renamed on success

	trailer := make([]byte, trailerSize)
	binary.LittleEndian.PutUint64(trailer, uint64(len(payload)))
	copy(trailer[8:], magic)

	if _, err := io.Copy(tmp, exe); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("writing output: %w", err)
	}
	if _, err := tmp.Write(append(payload, trailer...)); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("writing output: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("writing output: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil { //nolint:gosec // the output is an executable
		return "", fmt.Errorf("writing output: %w", err)
	}
	if err := os.Rename(tmp.Name(), absOutput); err != nil {
		return "", fmt.Errorf("writing output: %w", err)
	}
	return absOutput, nil
}

func sameFile(a, b string) (bool, error) {
	ai, err := os.Stat(a)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking %s: %w", a, err)
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false, fmt.Errorf("checking %s: %w", b, err)
	}
	return os.SameFile(ai, bi), nil
}

func read(exePath string) (*Bundle, error) {
	f, err := os.Open(exePath) //nolint:gosec // exePath is os.Executable, not user input
	if err != nil {
		return nil, fmt.Errorf("opening executable: %w", err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("reading executable: %w", err)
	}
	size := info.Size()
	if size < int64(trailerSize) {
		return nil, nil
	}

	trailer := make([]byte, trailerSize)
	if _, err := f.ReadAt(trailer, size-int64(trailerSize)); err != nil {
		return nil, fmt.Errorf("reading executable: %w", err)
	}
	if string(trailer[8:]) != magic {
		return nil, nil
	}

	length := binary.LittleEndian.Uint64(trailer)
	limit := uint64(size) - uint64(trailerSize) //nolint:gosec // size is at least trailerSize here
	if length > limit {
		return nil, errors.New("corrupt app bundle: length exceeds file size")
	}
	payload := make([]byte, length)
	if _, err := f.ReadAt(payload, int64(limit-length)); err != nil { //nolint:gosec // bounded by file size above
		return nil, fmt.Errorf("reading app bundle: %w", err)
	}

	var bundle Bundle
	if err := json.Unmarshal(payload, &bundle); err != nil {
		return nil, fmt.Errorf("corrupt app bundle: %w", err)
	}
	return &bundle, nil
}
