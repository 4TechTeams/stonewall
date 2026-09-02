package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootCmd(t *testing.T) {
	t.Run("no args prints help", func(t *testing.T) {
		cmd := newRootCmd()
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if !strings.Contains(stdout.String(), "USAGE") {
			t.Errorf("stdout missing USAGE:\n%s", stdout.String())
		}
	})

	t.Run("--help prints help", func(t *testing.T) {
		cmd := newRootCmd()
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"--help"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if !strings.Contains(stdout.String(), "USAGE") {
			t.Errorf("stdout missing USAGE:\n%s", stdout.String())
		}
	})

	t.Run("-p with no value errors", func(t *testing.T) {
		cmd := newRootCmd()
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-p"})
		if err := cmd.Execute(); err == nil {
			t.Error("expected an error for -p with no value")
		}
	})

	t.Run("first run refused writes no policy", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(wd)
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}
		stdin = strings.NewReader("n\n")
		defer func() { stdin = os.Stdin }()

		cmd := newRootCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"--plain", "-n", "sh", "-c", "true"})
		err = cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "aborted") {
			t.Fatalf("expected an abort error, got %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, ".stonewall.yml")); !os.IsNotExist(err) {
			t.Error("policy written after the user said no")
		}
	})

	t.Run("interspersed off passes --weird through", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(wd)
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}

		cmd := newRootCmd()
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"-n", "sh", "-c", "--weird"})
		err = cmd.Execute()
		// interspersed-off means "--weird" is a positional arg for the agent, never a
		// stonewall flag; cobra/pflag would report an "unknown flag" error if it were parsed.
		if err != nil && strings.Contains(err.Error(), "unknown flag") {
			t.Errorf("--weird was parsed as a stonewall flag: %v", err)
		}
	})
}
