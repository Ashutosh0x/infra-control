//go:build preview

package cli

// Error helpers for the commands that have no implementation.
//
// They live behind the same build tag as their callers so a default build
// carries neither the commands nor the machinery for explaining their absence.

// notImplemented reports that a capability exists in the data model but has no
// working implementation in this build.
//
// This returns an error rather than printing a plausible-looking result on
// purpose. A CLI that prints invented inventory is worse than one that admits
// the gap: the invented output looks exactly like a real answer and will be
// acted on.
func notImplemented(capability, tracking string) error {
	return failf(ExitError,
		"%s is not implemented in this build.\n"+
			"  This command has no data source wired up, and printing a placeholder result\n"+
			"  would be indistinguishable from a real answer.\n"+
			"  Status and design notes: %s", capability, tracking)
}

// requiresBackend reports that a command needs the control-plane server, which
// has not been configured.
func requiresBackend(command string) error {
	return failf(ExitConfig,
		"%s requires a running infra-control server.\n"+
			"  Set one with --server, or the INFRACTL_SERVER environment variable.\n"+
			"  To run one locally: docker compose -f deployments/docker/docker-compose.yaml up", command)
}
