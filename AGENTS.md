# Repository Guidelines

## Project Structure & Module Organization
`cmd/bot/main.go` is the application entrypoint. Core packages live under `internal/`: `bot` for Telegram handlers and auth, `worker` for job execution, `transcribe` for local/OpenAI/Mistral transcription backends, `summary` for summary generation, `queue` for SQLite-backed jobs and settings, `config` for loading `config.yaml`, and `cleanup` for retention tasks. Runtime data belongs in `data/` and logs in `logs/`. Design notes and plans are kept in `docs/superpowers/`.

## Build, Test, and Development Commands
Use the `Makefile` for routine work:

- `make build` builds `./cmd/bot` into `bin/bot`.
- `make build-pi` cross-compiles an ARM64 binary for Raspberry Pi.
- `make run` builds and starts the bot with `config.yaml`.
- `make test` runs `go test ./... -v`.
- `make init-dirs` creates `data/audio`, `data/output`, and `logs`.
- `make clean` removes build artifacts and generated audio/output files.

For quick iteration, `go test ./internal/...` is fine when you only touch internal packages.

## Coding Style & Naming Conventions
This is a Go 1.23 project. Format code with `gofmt` and keep imports organized with standard Go tooling. Follow existing package naming: short, lowercase directories under `internal/`, exported identifiers in PascalCase, unexported helpers in camelCase, and tests in `*_test.go`. Prefer constructor-style helpers such as `NewGenerator` and `NewWhisperTranscriber` when adding new components.

## Testing Guidelines
Tests are package-local and colocated with implementation files, for example `internal/worker/worker_test.go`. Add focused unit tests for new logic and regressions, especially around queue persistence, config parsing, and transcription mode selection. Run `make test` before opening a PR. When touching filesystem behavior, keep tests isolated to temporary directories.

## Commit & Pull Request Guidelines
Recent history uses short, imperative commit subjects such as `Add job processing worker` and `Improve cleanup validation and test coverage`. Keep that style: one-line summary, present tense, capitalized verb first. Pull requests should describe the behavior change, note any config or operational impact, link the relevant issue or plan, and include sample bot output or logs when user-visible behavior changes.

## Security & Configuration Tips
Do not commit real secrets. Keep tokens in environment variables such as `TELEGRAM_BOT_TOKEN` and `OPENAI_API_KEY`, and use `config.yaml.example` as the template for local setup.
