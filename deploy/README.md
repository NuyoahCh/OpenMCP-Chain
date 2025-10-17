# Deployment Assets

The `deploy/` directory aggregates infrastructure-as-code and packaging assets
used to operate OpenMCP-Chain across environments.

## Structure

* `docker/` – Container build definitions for the daemon and supporting
  services.
* `helm/` – Kubernetes Helm charts for clustered deployments with configurable
  replicas, ingress, and secret management.
* `terraform/` – Infrastructure modules for provisioning cloud resources such as
  databases, message queues, and key management systems.

Each subdirectory includes environment-specific overrides and documentation.
