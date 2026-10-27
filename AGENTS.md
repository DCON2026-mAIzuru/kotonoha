# Repository Guidelines

## Project Structure & Module Organization

- `frontend/src/` contains the Next.js App Router, React components, hooks, API clients, and shared types; `frontend/public/` holds static assets.
- `backend/cmd/server/` starts the Go API. `backend/internal/` contains handlers, services, repositories, LLM integration, and prompts; `backend/pkg/` contains configuration and authentication helpers.
- `DL/` hosts the Python FastAPI image-diagnosis service and model files. `esp/` and `hardware/` contain device code and documentation.
- `supabase/migrations/` contains database migrations. Architecture and setup notes live in `docs/`; check `docs/AGENTS.md` when working there.

## Build, Test, and Development Commands

- `cd backend && go run ./cmd/server` starts the API; `go test ./...` runs its tests.
- `cd frontend && npm install && npm run dev` installs dependencies and starts the UI. Use `npm run lint` and `npm run build` to check and build it.
- `cd DL && pip install -r requirements.txt && uvicorn api:app --host 127.0.0.1 --port 8090` starts image diagnosis; use a virtual environment.
- Copy the relevant `.env.example` values into local environment files before running services. See the root and `DL/` READMEs for setup and integration details.

## Coding Style & Naming Conventions

Format Go changes with `gofmt`; use Go's standard exported and unexported naming conventions. Follow existing TypeScript and React patterns: strict TypeScript, two-space indentation, kebab-case filenames, PascalCase components, and `use`-prefixed hooks. Run the frontend ESLint command before submitting UI changes. Keep migrations timestamp-prefixed and descriptive, matching existing SQL filenames.

## Testing Guidelines

Go tests use the standard `testing` package and live beside implementation files as `*_test.go`, with `Test...` functions. Add focused tests for changed behavior and run `go test ./...` from `backend/`. No repository-wide coverage threshold or frontend test command is documented; for frontend changes run lint and build, and manually check affected flows.

## Commit & Pull Request Guidelines

Recent history mixes short imperative subjects with prefixes such as `fix:`; use a concise, descriptive subject, optionally with a conventional prefix. PRs should explain the change and verification performed, link any relevant issue, and include screenshots for visible UI changes. Call out configuration or migration changes explicitly.

## Security & Configuration

Never commit credentials or local `.env` files. Keep secrets in environment variables, and document new configuration in the appropriate example file. Review migrations and external-service settings before deployment.
