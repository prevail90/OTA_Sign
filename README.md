# OTA Sign

OTA Sign is a forms control portal for Moodle-backed users and DocuSeal-backed signing.

The intended architecture is:

```text
Moodle LMS
  -> OTA Sign Connector Moodle plugin
  -> signed launch token
  -> OTA Sign backend
  -> OTA Sign frontend
  -> DocuSeal API/webhooks
```

Moodle remains the identity source. DocuSeal remains the signing engine. OTA Sign is the control layer for UIC-scoped access, form visibility, submissions, commander signatures, status tracking, downloads, and commander access lifecycle.

## Repository Layout

```text
backend/                    Go API service
frontend/                   React + Vite dashboard
moodle/local_otasignconnector/
                            Moodle local plugin skeleton
docs/                       Architecture and API notes
```

## MVP Goal

The first milestone is:

```text
Logged-in Moodle user clicks Open OTA Sign
-> Moodle plugin signs launch payload
-> OTA Sign backend validates token
-> Backend creates session
-> User lands on the right dashboard
```

After that, the DocuSeal integration can be added behind the backend API without exposing DocuSeal secrets to the browser.

## Backend Quick Start

Go is required to run the backend.

```bash
cd backend
cp .env.example .env
go run ./cmd/server
```

## Frontend Quick Start

```bash
cd frontend
npm install
npm run dev
```

By default the frontend expects the backend at `http://localhost:8080`.

## Production Images

Production is intended to run from prebuilt GHCR images with no Portainer-side
build step:

```text
ghcr.io/YOUR_GITHUB_ORG/otasign-backend:latest
ghcr.io/YOUR_GITHUB_ORG/otasign-frontend:latest
```

Use `docker-compose.prod.example.yml` as the Portainer stack starting point.
See `docs/operations.md` for deployment, backup, restore, monitoring, and
secret rotation procedures.
Use `.env.prod.example` as the production stack variable template.
The frontend image is runtime-configurable with:

```text
OTASIGN_API_BASE_URL
OTASIGN_MOODLE_LOGIN_URL
```

The backend image is a compiled Go binary and expects production env vars such
as `DATABASE_URL`, `FRONTEND_URL`, `MOODLE_OTA_SIGN_LAUNCH_URL`,
`DOCUSEAL_API_KEY`, and webhook secrets.

## Notification Webhook Demo

After setting `NOTIFICATION_WEBHOOK_URL` in `backend/.env`, send sample n8n
payloads with:

```bash
node scripts/send-demo-notification.js
```

The script sends one `commander_signature_requested` payload and one
`submission_completed` payload. If `NOTIFICATION_WEBHOOK_SECRET` is set, it
signs requests with `X-OTA-Signature`.

## Moodle Plugin

The Moodle plugin skeleton lives at:

```text
moodle/local_otasignconnector
```

Install it into Moodle as:

```text
local/otasignconnector
```

Its user-facing name is **OTA Sign Connector**.
