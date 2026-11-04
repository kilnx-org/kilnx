package build

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fakeExe(t *testing.T, dir string) string {
	t.Helper()
	exe := filepath.Join(dir, "kilnx-fake")
	if err := os.WriteFile(exe, []byte("\x7fELF fake runtime"), 0o600); err != nil {
		t.Fatal(err)
	}
	return exe
}

func TestWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	exe := fakeExe(t, dir)
	want := Bundle{
		Source:    "page /\n  html\n    <h1>`quoted` \x00 ok</h1>\n",
		Manifests: map[string]string{"user": "field nick: text\n"},
	}

	out, err := write(exe, want, filepath.Join(dir, "app.kilnx"), filepath.Join(dir, "myapp"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := read(out)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got == nil {
		t.Fatal("read returned no bundle")
		return
	}
	if got.Source != want.Source || got.Manifests["user"] != want.Manifests["user"] {
		t.Errorf("bundle mismatch: %+v", got)
	}

	raw, err := os.ReadFile(out) //nolint:gosec // test temp path
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "\x7fELF fake runtime") {
		t.Error("output does not start with the executable")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(out)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o100 == 0 {
			t.Errorf("output is not executable: %v", info.Mode())
		}
	}
}

func TestReadPlainExecutable(t *testing.T) {
	dir := t.TempDir()
	got, err := read(fakeExe(t, dir))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != nil {
		t.Errorf("expected no bundle, got %+v", got)
	}

	short := filepath.Join(dir, "short")
	if err := os.WriteFile(short, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := read(short); err != nil || got != nil {
		t.Errorf("short file: got %+v, %v", got, err)
	}
}

func TestReadCorruptBundle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt")
	trailer := make([]byte, 8)
	trailer[0] = 0xff
	trailer[3] = 0xff
	if err := os.WriteFile(path, append(trailer, magic...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := read(path); err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Errorf("expected corrupt bundle error, got %v", err)
	}
}

func TestWriteRefusesToOverwriteSourceOrExecutable(t *testing.T) {
	dir := t.TempDir()
	exe := fakeExe(t, dir)
	src := filepath.Join(dir, "app")
	if err := os.WriteFile(src, []byte("page /\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := write(exe, Bundle{}, src, src); err == nil || !strings.Contains(err.Error(), "source") {
		t.Errorf("expected source overwrite error, got %v", err)
	}
	if _, err := write(exe, Bundle{}, src, exe); err == nil || !strings.Contains(err.Error(), "executable") {
		t.Errorf("expected executable overwrite error, got %v", err)
	}
}

func TestWriteDefaultOutputName(t *testing.T) {
	dir := t.TempDir()
	exe := fakeExe(t, dir)
	t.Chdir(dir)

	out, err := write(exe, Bundle{Source: "x"}, "shop.kilnx", "")
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	want := "shop"
	if runtime.GOOS == "windows" {
		want = "shop.exe"
	}
	if filepath.Base(out) != want {
		t.Errorf("got %s, want %s", out, want)
	}
}
