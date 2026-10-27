# Patient Portal

A medical portal for the information security assignment, with a Go/Gin API,
PostgreSQL, and a Svelte + TypeScript frontend.

## Run locally

Use Go 1.26 and Node.js 24 LTS. The frontend also supports Node.js 20.19+ or
22.12+ as specified in `frontend/package.json`.

1. Create a local PostgreSQL database.
2. Copy `backend/.env.example` to `backend/.env`, set `DATABASE_URL` and
   `JWT_SECRET`, and generate `FILE_ENCRYPTION_KEY` once with `openssl rand -hex 32`.
   Keep the key across restarts so existing files can still be decrypted.
3. In `backend/`, run `go run .`.
4. In `frontend/`, run `npm ci` and `npm run dev`.
5. Open http://localhost:5173.

The Vite development proxy forwards `/api` to `http://localhost:8080`. Keep
`SERVER_PORT=8080` or update the proxy target in `frontend/vite.config.ts`.
Production hosting must serve `frontend/dist/` and route `/api` to the Go API.
Database credentials and cryptographic keys stay in the ignored backend `.env`.

## Current behavior

- Choose the patient or doctor illustration to sign in. Keyboard navigation and
  reduced-motion preferences are supported; narrow screens stack the form.
- Public registration accepts name, username, and password and always creates a
  patient. Doctor accounts must be provisioned by the hospital.
- Login uses the backend's HttpOnly cookie. `/api/users/me` supplies the actual
  identity and role after login and on reload. Choosing a character grants no
  permissions. Credentials and tokens are never saved in browser storage.
- The current frontend ends at the account confirmation. Records, hospital-wide
  doctor access, and server-side logout are not implemented. Existing file API
  endpoints remain owner-only.

## Structure and checks

`frontend/src/` contains `App.svelte`, `app.css`, and `main.ts`. It uses native
`fetch` and CSS; no router, UI kit, or state library is needed for this screen.

```sh
# In frontend/
npm run check
npm run build
npm run check:api  # Optional: backend + database + frontend must be running

# In backend/
go test -race ./...
go vet ./...
```

The optional API check creates one synthetic patient per run and verifies the
real proxy, registration, cookie, and `/me` contract. It does not run in CI.

GitHub Actions checks both projects. See [API contracts](docs/API.md) and
[CI setup](docs/CI.md). Generated illustration and logo prompts are recorded in
[illustration notes](docs/ILLUSTRATION.md) and [logo notes](docs/LOGO.md).
The locally hosted Inter font includes its OFL license in `frontend/public/fonts/`.

Commit each completed change with its relevant checks so the team can follow
and review development.
