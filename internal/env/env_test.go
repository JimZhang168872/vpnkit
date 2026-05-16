package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderBash(t *testing.T) {
	got := Render(Options{Shell: "bash", Port: 7890, NoProxy: "localhost,127.0.0.1"})
	for _, want := range []string{
		"export http_proxy=http://127.0.0.1:7890",
		"export https_proxy=http://127.0.0.1:7890",
		"export all_proxy=socks5h://127.0.0.1:7890",
		"export no_proxy=localhost,127.0.0.1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRenderFish(t *testing.T) {
	got := Render(Options{Shell: "fish", Port: 7890})
	if !strings.Contains(got, "set -gx http_proxy http://127.0.0.1:7890") {
		t.Errorf("fish output: %s", got)
	}
}

func TestRenderUnset(t *testing.T) {
	got := Render(Options{Shell: "bash", Unset: true})
	for _, want := range []string{"unset http_proxy", "unset https_proxy", "unset all_proxy", "unset no_proxy"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q: %s", want, got)
		}
	}
}

func TestRenderBashWithAuth(t *testing.T) {
	got := Render(Options{Shell: "bash", Port: 7890, User: "alice", Pass: "p4ss"})
	for _, want := range []string{
		"export http_proxy=http://alice:p4ss@127.0.0.1:7890",
		"export https_proxy=http://alice:p4ss@127.0.0.1:7890",
		"export all_proxy=socks5h://alice:p4ss@127.0.0.1:7890",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRenderURLEscapesPassword(t *testing.T) {
	got := Render(Options{Shell: "bash", Port: 7890, User: "a@b", Pass: "p:@#%"})
	// password chars must be percent-encoded so the URL parses.
	if strings.Contains(got, "p:@#%") {
		t.Errorf("password not URL-encoded:\n%s", got)
	}
	if !strings.Contains(got, "@127.0.0.1:7890") {
		t.Errorf("host part missing:\n%s", got)
	}
}

func TestWriteNetrcCreatesEntryWith0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".netrc")
	if err := WriteNetrc(path, "127.0.0.1", "alice", "p4ss"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %v, want 0600", info.Mode().Perm())
	}
	data, _ := os.ReadFile(path)
	s := string(data)
	for _, want := range []string{"machine 127.0.0.1", "login alice", "password p4ss"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}

func TestWriteNetrcPreservesNonStandardForeignEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".netrc")
	pre := "machine github.com\n  login alice\n  password ghp_xyz\n  account team\n" +
		"machine npm.example.com login bob password pp\n"
	if err := os.WriteFile(path, []byte(pre), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteNetrc(path, "127.0.0.1", "u", "p"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	s := string(data)
	for _, want := range []string{
		"machine github.com",
		"login alice",
		"password ghp_xyz",
		"account team",
		"machine npm.example.com login bob password pp",
		"machine 127.0.0.1 login u password p",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}

func TestWriteNetrcChmod0600OnExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".netrc")
	if err := os.WriteFile(path, []byte("machine x login y password z\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteNetrc(path, "127.0.0.1", "u", "p"); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %v, want 0600", info.Mode().Perm())
	}
}

func TestWriteNetrcReplacesExistingEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".netrc")
	pre := "machine example.com login u password p\nmachine 127.0.0.1 login old password old\n"
	if err := os.WriteFile(path, []byte(pre), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteNetrc(path, "127.0.0.1", "alice", "newpass"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	s := string(data)
	if !strings.Contains(s, "machine example.com login u password p") {
		t.Errorf("foreign entry lost:\n%s", s)
	}
	if strings.Contains(s, "password old") {
		t.Errorf("old entry not replaced:\n%s", s)
	}
	if !strings.Contains(s, "password newpass") {
		t.Errorf("new entry missing:\n%s", s)
	}
}
