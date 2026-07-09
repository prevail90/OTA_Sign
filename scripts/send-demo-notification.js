#!/usr/bin/env node

const crypto = require('node:crypto');
const fs = require('node:fs');

const env = loadEnv('backend/.env');
const webhookUrl = env.NOTIFICATION_WEBHOOK_URL;

if (!webhookUrl) {
  console.error('NOTIFICATION_WEBHOOK_URL is not set in backend/.env');
  process.exit(1);
}

const portalUrl = trimRight(env.FRONTEND_URL || 'https://sign.example.com', '/');
const launchUrl = env.MOODLE_OTA_SIGN_LAUNCH_URL || 'https://moodle.example.com/local/otasignconnector/launch.php';

const baseSubmission = {
  id: 'demo-submission-id',
  template_id: 'demo-template-id',
  template_name: "Commander's Interview",
  soldier_name: 'Demo Soldier',
  soldier_email: 'demo.soldier@example.mil',
  soldier_dod_id: '1111222233',
  uic: 'WABC12',
  status: 'pending',
  docuseal_submission_id: 'demo-docuseal-submission-id',
};

const events = [
  {
    event_type: 'commander_signature_requested',
    occurred_at: new Date().toISOString(),
    submission: baseSubmission,
    recipients: [
      {
        name: 'Demo Commander',
        email: 'demo.commander@example.mil',
        role: 'Commander',
        uic: 'WABC12',
      },
    ],
    docuseal: {
      id: 'demo-soldier-submitters-id',
      submission_id: 'demo-docuseal-submission-id',
      role: 'Soldier',
      status: 'completed',
    },
    metadata: {
      portal_url: portalUrl,
      launch_url: launchUrl,
      demo: 'true',
    },
  },
  {
    event_type: 'submission_completed',
    occurred_at: new Date().toISOString(),
    submission: {
      ...baseSubmission,
      status: 'complete',
    },
    recipients: [
      {
        name: 'Demo Soldier',
        email: 'demo.soldier@example.mil',
        role: 'Soldier',
        uic: 'WABC12',
      },
      {
        name: 'Demo Commander',
        email: 'demo.commander@example.mil',
        role: 'Commander',
        uic: 'WABC12',
      },
    ],
    docuseal: {
      id: 'demo-docuseal-submission-id',
      status: 'completed',
      combined_document_url: 'https://docuseal.example.com/demo-completed.pdf',
    },
    metadata: {
      portal_url: portalUrl,
      launch_url: launchUrl,
      demo: 'true',
    },
  },
];

sendEvents().catch((error) => {
  console.error(error);
  process.exit(1);
});

async function sendEvents() {
  for (const event of events) {
    const body = JSON.stringify(event);
    const headers = {
      accept: 'application/json',
      'content-type': 'application/json',
    };

    if (env.NOTIFICATION_WEBHOOK_SECRET) {
      const timestamp = Math.floor(Date.now() / 1000).toString();
      const signature = crypto
        .createHmac('sha256', env.NOTIFICATION_WEBHOOK_SECRET)
        .update(`${timestamp}.${body}`)
        .digest('hex');
      headers['x-ota-signature'] = `${timestamp}.${signature}`;
    }

    const response = await fetch(webhookUrl, {
      method: 'POST',
      headers,
      body,
    });

    const responseText = await response.text();
    console.log(
      JSON.stringify({
        event_type: event.event_type,
        status: response.status,
        ok: response.ok,
        response_length: responseText.length,
      }),
    );
  }
}

function loadEnv(path) {
  const result = {};
  const text = fs.existsSync(path) ? fs.readFileSync(path, 'utf8') : '';
  for (const line of text.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) {
      continue;
    }
    const index = trimmed.indexOf('=');
    if (index < 0) {
      continue;
    }
    result[trimmed.slice(0, index)] = unquote(trimmed.slice(index + 1));
  }
  return result;
}

function unquote(value) {
  if (
    (value.startsWith('"') && value.endsWith('"')) ||
    (value.startsWith("'") && value.endsWith("'"))
  ) {
    return value.slice(1, -1);
  }
  return value;
}

function trimRight(value, char) {
  while (value.endsWith(char)) {
    value = value.slice(0, -1);
  }
  return value;
}
