package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/1node/airon/internal/bootstrap"
)

const (
	exitSuccess = 0
	exitRuntime = 1
	exitUsage   = 2
)

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
	if len(args) == 0 || args[0] != "run" {
		printUsage()
		return exitUsage
	}

	runFlags := flag.NewFlagSet("run", flag.ContinueOnError)
	runFlags.SetOutput(os.Stderr)

	configPath := runFlags.String("config", "", "Path to YAML config")
	inputValue := runFlags.String("input", "", `Sets template variable "input"`)
	stream := runFlags.Bool("stream", false, "Stream assistant text to stdout")
	var vars varFlags
	runFlags.Var(&vars, "var", "Additional template variable (key=value)")

	if err := runFlags.Parse(args[1:]); err != nil {
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
		Stream: *stream,
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

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  airon run --config ./airon.yaml [--input \"text\"] [--stream] [--var key=value]")
}
