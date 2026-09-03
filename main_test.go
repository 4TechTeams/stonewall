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

	t.Run("subcommand help comes from its definition", func(t *testing.T) {
		var stdout bytes.Buffer
		cmd := newRootCmd()
		cmd.SetOut(&stdout)
		cmd.SetArgs([]string{"policy", "include", "--help"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"USAGE", "stonewall policy include <url|path>", "GLOBAL OPTIONS", "--policy FILE"} {
			if !strings.Contains(stdout.String(), want) {
				t.Errorf("subcommand help missing %q:\n%s", want, stdout.String())
			}
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

	t.Run("first run writes the policy without asking", func(t *testing.T) {
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
		stdin = strings.NewReader("n\n") // decline the remote includes the scaffold pulls in
		defer func() { stdin = os.Stdin }()

		cmd := newRootCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"--plain", "-n", "sh", "-c", "true"})
		_ = cmd.Execute() // fails on the untrusted includes; the scaffold must be on disk regardless
		b, err := os.ReadFile(filepath.Join(dir, ".stonewall.yml"))
		if err != nil || !strings.Contains(string(b), "include:") {
			t.Fatalf("scaffold not written: %v", err)
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
