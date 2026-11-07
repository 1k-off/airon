package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/1k-off/airon/internal/bootstrap"
	"github.com/1k-off/airon/internal/config"
	"github.com/1k-off/airon/internal/mcp"
)

const (
	exitSuccess = 0
	exitRuntime = 1
	exitUsage   = 2
)

func init() {
	// Keep MCP client version in sync with the binary version string.
	if version != "" && version != "dev" {
		mcp.ClientVersion = version
	}
}

type varFlags map[string]string

func (v *varFlags) String() string {
	if v == nil {
		return ""
	}
	parts := make([]string, 0, len(*v))
	for key, value := range *v {
		parts = append(parts, key+"="+value)
	}
	return strings.Join(parts, ",")
}

func (v *varFlags) Set(value string) error {
	key, val, ok := strings.Cut(value, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return fmt.Errorf("invalid --var %q, expected key=value", value)
	}
	if *v == nil {
		*v = make(map[string]string)
	}
	(*v)[key] = val
	return nil
}

func main() {
	os.Exit(runCLI(os.Args[1:]))
}

func runCLI(args []string) int {
	if len(args) == 0 {
		printUsage()
		return exitUsage
	}

	switch args[0] {
	case "run":
		return runRun(args[1:])
	case "check":
		return runCheck(args[1:])
	case "version", "--version", "-version":
		fmt.Fprintf(os.Stdout, "airon %s (commit=%s date=%s)\n", version, commit, date)
		return exitSuccess
	default:
		printUsage()
		return exitUsage
	}
}

func runRun(args []string) int {
	runFlags := flag.NewFlagSet("run", flag.ContinueOnError)
	runFlags.SetOutput(os.Stderr)

	configPath := runFlags.String("config", "", "Path to YAML config")
	inputValue := runFlags.String("input", "", `Sets template variable "input"`)
	stream := runFlags.Bool("stream", false, "Stream assistant text to stdout")
	noHealthchecks := runFlags.Bool("no-healthchecks", false, "Disable healthchecks.io notifier for this run")
	var vars varFlags
	runFlags.Var(&vars, "var", "Additional template variable (key=value)")

	if err := runFlags.Parse(args); err != nil {
		return exitUsage
	}
	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "error: --config is required")
		printUsage()
		return exitUsage
	}

	input := make(map[string]string, len(vars)+1)
	for key, value := range vars {
		input[key] = value
	}
	if *inputValue != "" {
		input["input"] = *inputValue
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	app, err := bootstrap.NewAppWithOptions(ctx, *configPath, input, bootstrap.AppOptions{
		Stream:         *stream,
		NoHealthchecks: *noHealthchecks,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "runtime error: %v\n", err)
		return exitRuntime
	}
	defer func() {
		if closeErr := app.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", closeErr)
		}
	}()

	result, err := app.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "runtime error: %v\n", err)
		return exitRuntime
	}

	fmt.Fprintf(os.Stdout, "run_id=%s steps=%d\n", result.RunID, len(result.Steps))
	return exitSuccess
}

func runCheck(args []string) int {
	checkFlags := flag.NewFlagSet("check", flag.ContinueOnError)
	checkFlags.SetOutput(os.Stderr)

	configPath := checkFlags.String("config", "", "Path to YAML config")
	strict := checkFlags.Bool("strict", false, "Fail if any ${ENV} references are unset")

	if err := checkFlags.Parse(args); err != nil {
		return exitUsage
	}
	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "error: --config is required")
		printUsage()
		return exitUsage
	}

	report, err := config.Check(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "check failed: %v\n", err)
		return exitRuntime
	}

	fmt.Fprintf(os.Stdout, "ok: config valid\n")
	fmt.Fprintf(os.Stdout, "  provider=%s model=%s\n", report.Config.Provider.Type, report.Config.Provider.Model)
	fmt.Fprintf(os.Stdout, "  prompts=%d skills=%d mcp_servers=%d\n",
		len(report.Config.Prompts), len(report.Config.Skills), len(report.Config.MCPServers))

	for _, w := range report.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	if len(report.MissingEnv) > 0 {
		fmt.Fprintf(os.Stderr, "warning: unset environment variables: %s\n", strings.Join(report.MissingEnv, ", "))
		fmt.Fprintln(os.Stderr, "hint: set them before `airon run`, or re-run with --strict to fail closed")
		if *strict {
			return exitRuntime
		}
	}

	return exitSuccess
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  airon run     --config ./airon.yaml [--input \"text\"] [--stream] [--no-healthchecks] [--var key=value]")
	fmt.Fprintln(os.Stderr, "  airon check   --config ./airon.yaml [--strict]")
	fmt.Fprintln(os.Stderr, "  airon version")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "check validates YAML structure and paths without calling LLM or MCP.")
	fmt.Fprintln(os.Stderr, "Missing ${ENV} refs are reported as warnings unless --strict is set.")
}
