# OTA Sign Connector

Moodle local plugin component:

```text
local_otasignconnector
```

Display name:

```text
OTA Sign Connector
```

## MVP Purpose

The plugin provides a secure launch from Moodle into the OTA Sign portal.

```text
Moodle user clicks a manually configured OTA Sign navbar link
-> plugin builds user context
-> plugin signs context with HMAC-SHA256
-> plugin redirects to OTA Sign backend /launch
```

## Required Settings

- OTA Sign launch URL, for example `https://sign.example.mil/launch`
- Launch signing secret, matching `MOODLE_LAUNCH_SIGNING_SECRET` in the backend
- UIC custom profile field shortname
- DoD ID custom profile field shortname
- Rank custom profile field shortname. OTA Sign derives pay grade from rank using its `dod_paygrades` database table.

## Moodle Navbar Link

Add a Moodle custom menu/navbar link that points to the plugin launch page:

```text
OTA Sign|/local/otasignconnector/launch.php" target="_blank|Digitally sign your documents for OTA.
```

Do not point the menu link directly at the OTA Sign backend `/launch` endpoint;
the plugin launch page creates the signed token required by the backend.

## Install Path

Copy this directory into Moodle as:

```text
local/otasignconnector
```
