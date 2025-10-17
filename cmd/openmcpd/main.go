package main

import (
	"context"
	"log"
)

// main is the entrypoint for the OpenMCP daemon. It currently wires up
// the bare minimum runtime context and logs a startup message. Future
// revisions will extend this to initialize configuration, storage, web3
// adapters, and the agent orchestrator.
func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		log.Fatalf("openmcpd failed: %v", err)
	}
}

// run bootstraps the daemon runtime. The placeholder implementation gives
// the project a compilable binary while the remaining subsystems are being
// implemented. Each subsystem should expose an initialization hook that is
// invoked here in a well-defined order.
func run(ctx context.Context) error {
	log.Println("OpenMCP daemon initialized")
	// TODO: initialize configuration, storage providers, agent runtime,
	// API servers, and blockchain clients.
	<-ctx.Done()
	return ctx.Err()
}
