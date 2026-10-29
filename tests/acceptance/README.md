# Disposable acceptance tests

Run `python3 tests/acceptance/run.py` with Docker and Terraform/OpenTofu installed.
Set `TF_ACC_TERRAFORM_PATH` to select the CLI. The script creates a dedicated Docker
network, publishes only the WebUI to loopback, seeds a random
password using the supported PBKDF2 config format, and seeds a random API key.
Host validation, CSRF protection, and localhost authentication remain enabled.
The network, container and temporary config are removed even on test failure.

The image is pinned in run.py by multi-platform manifest digest. The runner
verifies and prints both application and Web API versions before running tests.
Credentials are masked under GitHub Actions and are not printed locally. Do not
use shell tracing or enable HTTP body logging. Tests receive only temporary
credentials through environment variables and require the runner's container name.
DHT, PeX, LSD and port forwarding start disabled. The only torrent fixture is
stopped and has no tracker. Tests run entirely against the disposable server
with generated credentials and local fixture data.

Plugin Framework acceptance tests cover API-key and session preferences lifecycles,
partial ownership, drift, import, no-op plans, category rename/replacement and
invalid credentials. Unit tests additionally exercise malformed responses,
auth failures, cancellation, timeouts and deletion guards.
