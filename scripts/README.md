# Scripts

The `scripts/` directory is reserved for automation utilities that streamline
local development, testing, and deployment workflows. Scripts should be
idempotent, well-documented, and safe to run repeatedly.

Suggested additions include:

* `bootstrap.sh` – Provision local dependencies (MySQL, Redis, test blockchain).
* `migrate.sh` – Apply database migrations across environments.
* `codegen.sh` – Generate protobuf/SDK artifacts.

Ensure executable scripts include a shebang and descriptive comments.
