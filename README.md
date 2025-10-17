# OpenMCP-Chain

OpenMCP-Chain is a decentralized protocol stack that bridges blockchain
infrastructure with large-model-driven autonomous agents. The project aims to
provide a production-grade foundation for building trusted AI agents capable of
interacting with Web3 systems through standardized workflows.

## Repository Structure

```
cmd/openmcpd/        # Daemon entrypoint and process wiring
internal/            # Core services and domain logic
  agent/             # Agent orchestration runtime
  api/               # External APIs (REST/gRPC) and handler plumbing
  config/            # Configuration management and environment handling
  llm/               # Large language model adapters and orchestration
  proofs/            # Cryptographic attestations and provenance tooling
  storage/mysql/     # Persistent data access layers backed by MySQL
  storage/redis/     # Caching and queue utilities backed by Redis
  web3/              # Blockchain client integrations and abstractions
pkg/sdk/             # External SDK for integrating with OpenMCP-Chain
deploy/              # Operational assets such as Docker, Helm, Terraform
scripts/             # Developer tooling and automation scripts
docs/                # In-depth project documentation and specifications
```

## Getting Started

1. Install Go 1.22 or newer.
2. Clone this repository and run `go build ./...` to verify the initial setup.
3. Review the documents in the `docs/` directory for architecture and roadmap
   details before contributing new functionality.

## Contributing

Contributions are welcome! Please open an issue or discussion thread to align
on design decisions before submitting pull requests. Adhere to Go best
practices, run linters/tests locally, and include documentation updates for new
features.

## License

This project is licensed under the terms of the MIT License. See `LICENSE` for
more details.
