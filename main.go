// Command stonewall launches an AI coding agent inside a kernel-enforced sandbox.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"

	"github.com/4TechTeams/stonewall/internal/policy"
	"github.com/4TechTeams/stonewall/internal/sandbox"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const version = "2.0.0-poc"

// out is stonewall's own stderr UI. Built plain in main before Execute, then refined by
// PersistentPreRunE once --plain is known. The initial value is used only on flag-parse errors,
// which never reach PersistentPreRunE; it is still TTY-aware.
var out ui

// exitError carries the process exit code out of RunE.
type exitError struct {
	code int
	err  error // nil when only the code matters (agent exit status)
}

func (e exitError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return fmt.Sprintf("exit %d", e.code)
}

func newRootCmd() *cobra.Command {
	var policyPath string
	var dryRun, plain bool

	cmd := &cobra.Command{
		Use:           "stonewall [options] <agent> [agent args...]",
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			out = newUI(os.Stderr, plain)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				fmt.Fprint(cmd.OutOrStdout(), newUI(cmd.OutOrStdout(), plain).help())
				return nil
			}
			return launch(args, policyPath, dryRun)
		},
	}
	cmd.SetVersionTemplate("stonewall {{.Version}}\n")
	cmd.Flags().SetInterspersed(false)
	// --policy and --plain are persistent so `stonewall policy update` takes them too.
	cmd.PersistentFlags().StringVarP(&policyPath, "policy", "p", "", "policy file (default: <project root>/"+policy.FileName+")")
	cmd.PersistentFlags().BoolVar(&plain, "plain", false, "no colour or formatting in stonewall's own output")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "print the sandbox command and exit")

	policyCmd := &cobra.Command{Use: "policy", Short: "Manage the project policy"}
	policyCmd.AddCommand(&cobra.Command{
		Use:   "update",
		Short: "Refresh cached remote policies",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return updatePolicies(policyPath) },
	})
	cmd.AddCommand(policyCmd)

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprint(cmd.OutOrStdout(), newUI(cmd.OutOrStdout(), plain).help())
	})

	return cmd
}

// newLoader builds the policy loader with stonewall's own review prompt, reporting a refresh
// accepted at the staleness prompt the way `policy update` does.
func newLoader() policy.Loader {
	return policy.Loader{
		Ask: func(title, body, question string) bool {
			out.block(title, body)
			return out.confirm(question)
		},
		Report: func(results []policy.UpdateResult) { reportUpdates(results) },
	}
}

// reportUpdates prints one row per refreshed policy and an error line per failure, and returns
// how many failed.
func reportUpdates(results []policy.UpdateResult) int {
	var rows [][2]string
	failed := 0
	for _, r := range results {
		if r.Status == "failed" {
			failed++
			continue
		}
		rows = append(rows, [2]string{r.Status, r.URL})
	}
	out.rows(rows...)
	for _, r := range results {
		if r.Status == "failed" {
			out.error(fmt.Errorf("%s: %w", r.URL, r.Err))
		}
	}
	return failed
}

// updatePolicies refreshes the cached remote policies of the project's policy file.
func updatePolicies(policyPath string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return exitError{1, err}
	}
	if policyPath == "" {
		policyPath = filepath.Join(policy.FindRoot(cwd), policy.FileName)
	}
	results, err := newLoader().Update(policyPath)
	failed := reportUpdates(results)
	if err != nil {
		if failed == 0 {
			return exitError{1, err}
		}
		return exitError{code: 1} // the failures are already reported
	}
	return nil
}

func main() {
	out = newUI(os.Stderr, false)
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		var ee exitError
		if errors.As(err, &ee) {
			if ee.err != nil {
				out.error(ee.err)
			}
			os.Exit(ee.code)
		}
		out.error(err) // cobra usage/flag error
		fmt.Fprintln(os.Stderr, "Run 'stonewall --help' for usage.")
		os.Exit(2)
	}
}

// launch runs the agent inside the sandbox: root discovery, policy scaffold and include resolution,
// sandbox build, backend selection, exec and signal handling. args[0] is the agent; the rest pass through.
func launch(args []string, policyPath string, dryRun bool) error {
	fail := func(err error) error { return exitError{1, err} }

	cwd, err := os.Getwd()
	if err != nil {
		return fail(err)
	}
	root := policy.FindRoot(cwd)
	home, err := os.UserHomeDir()
	if err != nil {
		return fail(err)
	}
	if same(root, home) || root == filepath.Dir(root) {
		return fail(fmt.Errorf("project root %s is your home directory or /; run stonewall inside a project (a directory holding .git or %s)", root, policy.FileName))
	}
	created := false
	if policyPath == "" {
		policyPath = filepath.Join(root, policy.FileName)
		if _, err := os.Stat(policyPath); errors.Is(err, os.ErrNotExist) {
			content := policy.Scaffold(args[0])
			out.block("stonewall will create "+policyPath, content)
			if !out.confirm("Write it?") {
				return fail(errors.New("aborted, no policy written"))
			}
			if err := policy.WriteScaffold(policyPath, content); err != nil {
				return fail(err)
			}
			created = true
		}
	}
	eff, localFiles, err := newLoader().Load(policyPath)
	if err != nil {
		return fail(err)
	}
	plan, err := sandbox.Build(eff, root, cwd, append([]string{policyPath}, localFiles...), args)
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(plan.BinDir)

	var cmd *exec.Cmd
	var backend string
	switch runtime.GOOS {
	case "linux":
		bwrap, err := exec.LookPath("bwrap")
		if err != nil {
			return fail(errors.New("bwrap not found: install the bubblewrap package"))
		}
		cmd = exec.Command(bwrap, sandbox.BwrapArgs(plan)...)
		backend = "bwrap"
	case "darwin":
		cmd = exec.Command("/usr/bin/sandbox-exec", sandbox.SeatbeltArgs(plan)...)
		backend = "sandbox-exec"
	default:
		return fail(fmt.Errorf("unsupported OS %s: stonewall runs on Linux and macOS", runtime.GOOS))
	}

	policyDisplay := policyPath
	if rel, err := filepath.Rel(root, policyPath); err == nil && !strings.HasPrefix(rel, "..") {
		policyDisplay = rel
	}
	if created {
		policyDisplay += " " + out.paint("32", "(created)")
	}
	if dryRun {
		backend += " (dry run)"
	}
	out.rows(
		[2]string{"project", root},
		[2]string{"policy", policyDisplay},
		[2]string{"sandbox", backend},
	)
	for _, w := range plan.Warnings {
		out.warn(w)
	}

	if dryRun {
		b, err := yaml.Marshal(eff)
		if err != nil {
			return fail(err)
		}
		out.block("effective policy", string(b))
		fmt.Println(shellJoin(cmd.Args))
		return nil
	}
	cmd.Env = plan.Env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	// Ctrl-C reaches the agent through the terminal (it already delivered SIGINT to the whole
	// foreground process group); stonewall forwards only termination signals.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	if err := cmd.Start(); err != nil {
		return fail(err)
	}
	go func() {
		for s := range sigs {
			if s != syscall.SIGINT { // the terminal already sent Ctrl-C to the agent's process group
				cmd.Process.Signal(s)
			}
		}
	}()
	err = cmd.Wait()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return exitError{code: 128 + int(ws.Signal())}
		}
		return exitError{code: exit.ExitCode()}
	}
	if err != nil {
		return fail(err)
	}
	return nil
}

// same reports whether two paths name the same directory after resolving symlinks.
func same(a, b string) bool {
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	return err1 == nil && err2 == nil && ra == rb
}

// plainArg matches args that never need shell quoting.
var plainArg = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// shellJoin quotes args so the line can be pasted into a shell.
func shellJoin(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		if !plainArg.MatchString(a) {
			a = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
		out[i] = a
	}
	return strings.Join(out, " ")
}
