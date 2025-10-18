# Secrets directory

Place secret material used by Docker Compose in this directory. The production
compose file expects the following entries:

* `openai-api-key` – contains the OpenAI compatible API token. The
  `openmcpd` service reads it through Docker secrets and exports it as the
  `OPENAI_API_KEY` environment variable.
* `mysql-password` – contains the password for the MySQL user configured in the
  compose file.

Files should contain the secret value without a trailing newline when possible.
Never commit real secrets to version control.
