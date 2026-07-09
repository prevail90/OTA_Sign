# OTA Sign Architecture

## System Roles

```text
Moodle
- Login and identity
- User profile data such as UIC
- Moodle roles/capabilities
- Launch point into OTA Sign

OTA Sign Backend
- Validates Moodle launch tokens
- Owns sessions and permissions
- Stores users, UIC roles, templates, submissions, signer state, and audit events
- Talks to DocuSeal API
- Receives DocuSeal webhooks
- Sends SMTP emails

OTA Sign Frontend
- Student "My Forms" dashboard
- Commander "Unit Forms" dashboard
- Start/view/sign/download actions through backend API only

DocuSeal
- Signing ceremony
- Signer authentication flow
- Audit trail
- Completed signed PDFs
```

## Launch Flow

```text
1. User logs into Moodle.
2. User clicks Open OTA Sign.
3. Moodle plugin builds a launch payload:
   - moodle_user_id
   - full_name
   - email
   - uic
   - roles/capabilities
   - issued_at
   - expires_at
4. Moodle signs the payload with HMAC-SHA256.
5. Moodle redirects to OTA Sign:
   /launch?token=<base64url(payload)>.<base64url(signature)>
6. Backend validates the token and creates a secure session.
7. Frontend loads /api/me and dashboard data from the backend.
```

## Security Rules

- The frontend never receives Moodle signing secrets, DocuSeal API keys, SMTP credentials, or database credentials.
- Every frontend request goes to the OTA Sign backend.
- Backend permission checks are UIC-scoped.
- A commander or commander representative can only see users and submissions for authorized UICs.
- A user can see their own submissions even when they also have commander permissions.
- DocuSeal webhooks must be verified before updating local state.

## Submission Status

OTA Sign should normalize DocuSeal status into these user-facing states:

```text
missing
pending
complete
canceled
failed
```

For commander dashboards, the backend should calculate the most current submission per:

```text
soldier_id + template_id
```

If no submission exists for that soldier/template, return `missing`.

## Commander Dashboard Shape

```text
Unit Forms
Search by soldier name, DoD ID, or form name

Soldier group
  Current submission for each relevant template
  Sign button when commander signer is waiting
  Download button when complete
  Missing state when no current submission exists
```

Commanders also have a normal "My Forms" view for their own submissions.
