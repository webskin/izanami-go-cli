You are an AI pair programmer working inside Claude Code.

Context:
- Project: Go CLI client for Izanami named `iz` (repository: `izanami-go-cli`).
- The CLI is a wrapper around the Izanami HTTP API, using Go.
- The CLI already uses a YAML config file at: ~/.config/iz/config.yaml

Current config file example:
----------------------------
The current config.yaml looks like:

base-url: http://localhost:9000
personal-access-token: 1895f9e9-...
timeout: 30
username: 97845a
verbose: false

There is already a Go `Config` struct (or equivalent) mapping these fields.
You must extend this existing config format in a backward-compatible way.

Existing `iz features check` behavior:
--------------------------------------
There is a command:

iz features check <feature-id>

It currently supports flags:

--client-id string        Client ID for authentication (env: IZ_CLIENT_ID)
--client-secret string    Client secret for authentication (env: IZ_CLIENT_SECRET)

Requirements:
- **Keep these flags exactly as they are.**
- **They must remain possible and have the highest priority.**

New goal (client keys management):
----------------------------------
Implement:
1) A way to register client credentials (client-id + client-secret) in the CLI config, either:
    - tenant-wide
    - or scoped to one or more projects inside a tenant.
2) A way for `iz features check` to automatically use these registered credentials when flags/env are not set.

New CLI command for registration
--------------------------------
Add a new subcommand under `iz config`:

# tenant-wide
iz config client-keys register --tenant <tenant-name>

# tenant + projects (repeatable flag)
iz config client-keys register --tenant <tenant-name> \
--project <project1> --project <project2>

Semantics:
- `--tenant <tenant-name>` (string, required)
    - Name of the Izanami tenant.
- `--project <project-name>` (string, repeatable, optional)
    - Can be specified zero or more times.
    - If `--project` is NOT provided at all: credentials are stored **tenant-wide** (default scope).
    - If `--project` is provided one or more times: credentials are stored per project for the given tenant.

Interactive credential input:
-----------------------------
When the register command runs, it must:
1. Prompt interactively for `client-id` (echo ON, normal input).
    - Example prompt: `Client ID: `
2. Prompt interactively for `client-secret` in **hidden / no-echo mode**.
    - Example prompt: `Client secret (hidden): `
    - Use a cross-platform way in Go to read a password without echoing it to the terminal
      (for example via `golang.org/x/term` or equivalent).
3. Do not print the secret back to the user at any point.
4. After successful registration, show a short confirmation message that **does not** display the secret.

Config structure (YAML + Go structs):
-------------------------------------
Extend the existing `Config` struct and YAML file with a new top-level field, for example:

- In Go (conceptually, adapt to the actual struct):

  type Config struct {
  BaseURL             string                             `yaml:"base-url"`
  PersonalAccessToken string                             `yaml:"personal-access-token"`
  Timeout             int                                `yaml:"timeout"`
  Username            string                             `yaml:"username"`
  Verbose             bool                               `yaml:"verbose"`
  ClientKeys          map[string]TenantClientKeysConfig  `yaml:"client-keys,omitempty"`
  }

  type TenantClientKeysConfig struct {
  ClientID     string                               `yaml:"client-id,omitempty"`
  ClientSecret string                               `yaml:"client-secret,omitempty"`
  Projects     map[string]ProjectClientKeysConfig   `yaml:"projects,omitempty"`
  }

  type ProjectClientKeysConfig struct {
  ClientID     string `yaml:"client-id,omitempty"`
  ClientSecret string `yaml:"client-secret,omitempty"`
  }

- In YAML, this should produce something like:

  base-url: http://localhost:9000
  personal-access-token: 1895f9e9-...
  timeout: 30
  username: 97845a
  verbose: false
  client-keys:
  my-tenant:
  client-id: some-tenant-client-id
  client-secret: some-tenant-client-secret
  projects:
  project-a:
  client-id: project-a-client-id
  client-secret: project-a-client-secret
  project-b:
  client-id: project-b-client-id
  client-secret: project-b-client-secret

Requirements:
- Keep the config backward-compatible:
    - Existing config.yaml files WITHOUT `client-keys` must still load correctly.
- Reuse the existing config loading / saving logic.
- Ensure the config file is updated on disk after running the register command.

Overwrite behavior:
-------------------
If credentials already exist for the same tenant / project combination:
- Overwrite them by default, but:
    - Before overwriting, print a warning like:
      "Client keys already exist for tenant '<tenant>' (projects: ...). Overwrite? [y/N]"
    - Ask for a confirmation.
    - Only overwrite if the user clearly confirms; otherwise, abort without changes.

Integration into `iz features check`:
-------------------------------------
Update `iz features check <feature-id>` to resolve credentials in this order of precedence:

1) **Command-line flags (highest priority)**
    - If `--client-id` and `--client-secret` are provided on the command line:
        - Use them.
        - They override everything else.
    - If only one of them is provided, treat this as an error and show a clear message.

2) **Environment variables**
    - If flags are not provided, check:
        - `IZ_CLIENT_ID`
        - `IZ_CLIENT_SECRET`
    - If both are set, use them.
    - If one is missing, treat this as “env not usable” and move on to config lookup.

3) **Config-based lookup (new behavior)**
    - If neither flags nor env provided valid credentials, then:
        - Check for `--tenant` and optional `--project` flags on `iz features check`.
            - If needed, add these flags to `features check` and keep them consistent with the register command:
                - `--tenant <tenant-name>` (required for config lookup)
                - `--project <project-name>` (repeatable, optional, same semantics as in register)
    - Resolution rules:
        - If `--tenant` is provided and **no** `--project` is given:
            - Look up tenant-level credentials at:
              config.ClientKeys[tenant].ClientID / ClientSecret
        - If `--tenant` is provided with one or more `--project` flags:
            - For simplicity, use the first `--project` value to look up:
              config.ClientKeys[tenant].Projects[project].ClientID / ClientSecret
            - If there is a better existing pattern in the codebase for handling multiple projects, follow it.
    - If no credentials are found in the config for the given tenant/project:
        - Print a clear error message, e.g.:
          "No client keys found for tenant '<tenant>' (project '<project>') in config. \
          Please run 'iz config client-keys register ...' or provide --client-id/--client-secret."
        - Exit with non-zero status.

Important:
- Do **not** change the existing API call logic of `features check` except for how client-id/client-secret are resolved.
- Make sure the final client-id/client-secret used by `features check` are traceable in code (for debugging), but never log the actual secret.

Non-interactive / scripting considerations:
-------------------------------------------
- Keep `--client-id` and `--client-secret` flags usable for non-interactive scripts.
- Their precedence over env and config must be clearly implemented and tested.

Error handling and UX:
----------------------
- If `--tenant`/`--project` are required for config lookup but missing, show a clear error and usage hint.
- If the config file cannot be read or written, print a clear error message including the file path (e.g. ~/.config/iz/config.yaml).
- Exit with non-zero status on errors.

Where to implement:
-------------------
- Locate the existing command tree for the CLI (probably in a `cmd/` or `cli/` package).
    - Add the `config` → `client-keys` → `register` command under that tree.
    - Extend the existing `features check` command to resolve credentials with the precedence described above.
- Locate the configuration package (e.g. `config` or equivalent) and extend the structs + read/write logic to include `client-keys`.

Testing:
--------
- Add unit tests for:
    - The new config structures and (un)marshalling to YAML.
    - `iz config client-keys register`:
        - tenant-only (no `--project`)
        - tenant + one project
        - tenant + multiple `--project` flags
        - overwrite confirmation behavior.
    - `iz features check` credential resolution precedence:
        - flags only
        - env only
        - config only (tenant-wide)
        - config only (tenant + project)
        - combinations where flags override env, and env override config.
    - Error cases (missing tenant/project for config lookup, missing credentials, etc.).
- Follow existing patterns for CLI and config tests.

Help / documentation:
---------------------
- Update the help text / usage for:
    - `iz config client-keys register --tenant TENANT [--project PROJECT ...]`
    - `iz features check` to mention:
        - `--client-id` / `--client-secret` with env vars.
        - Optional `--tenant` / `--project` for config-based credential lookup.
- Document the precedence: flags > env > config.

Implementation style:
---------------------
- Keep code idiomatic Go.
- Follow existing error-handling and logging conventions in the repo.
- Keep the code small and focused; if it grows, split pieces into helper functions where it makes sense.

Steps for you (Claude Code):
----------------------------
1. Inspect the repo structure to find:
    - CLI command definitions (including `features check`).
    - Configuration types and config file handling for ~/.config/iz/config.yaml.
2. Extend the `Config` struct and YAML mapping to support `client-keys`.
3. Implement `iz config client-keys register` with interactive prompting and config writing.
4. Extend `iz features check` to resolve credentials with the precedence:
   flags > env > config(tenant/project).
5. Add or update tests for both the new command and the resolution logic.
6. Show me the diff of the changes and briefly explain the design choices.
