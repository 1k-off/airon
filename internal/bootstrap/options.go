package bootstrap

// AppOptions configures optional runtime behavior for bootstrap.
type AppOptions struct {
	// Stream, when true, forces streaming of assistant output to stdout even
	// if the loaded config has runtime.stream disabled.
	Stream bool

	// NoHealthchecks disables the healthchecks.io notifier for this run,
	// regardless of whether the config supplies a notify.healthchecks.url.
	// This mirrors the CLI flag --no-healthchecks and is also implied when
	// the environment variable AIRON_HEALTHCHECKS is set to "0", "false",
	// or "no" (case-insensitive).
	NoHealthchecks bool
}
