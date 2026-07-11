<?php
// This file is part of Moodle - http://moodle.org/

$string['pluginname'] = 'OTA Sign Connector';
$string['open_otasign'] = 'OTA Sign';
$string['launch_url'] = 'OTA Sign launch URL';
$string['launch_url_desc'] = 'Backend launch endpoint that receives the signed launch token.';
$string['signing_secret'] = 'Launch signing secret';
$string['signing_secret_desc'] = 'Shared HMAC secret used to sign Moodle launch tokens. Store the same value in the OTA Sign backend.';
$string['uic_profile_field'] = 'UIC profile field shortname';
$string['uic_profile_field_desc'] = 'Moodle custom profile field shortname containing the user UIC.';
$string['dodid_profile_field'] = 'DoD ID profile field shortname';
$string['dodid_profile_field_desc'] = 'Moodle custom profile field shortname containing the user DoD ID.';
$string['rank_profile_field'] = 'Rank profile field shortname';
$string['rank_profile_field_desc'] = 'Moodle custom profile field shortname containing the user rank abbreviation. OTA Sign derives pay grade from this rank.';
$string['army_email_profile_field'] = 'Army email profile field shortname';
$string['army_email_profile_field_desc'] = 'Moodle custom profile field shortname containing the user @army.mil email address. If empty or not an @army.mil address, OTA Sign falls back to the regular Moodle email only when it is an @army.mil address.';
$string['missingconfig'] = 'OTA Sign Connector is missing its launch URL or signing secret.';
$string['otasignconnector:viewown'] = 'Launch OTA Sign and view own forms';
$string['otasignconnector:viewunit'] = 'View OTA Sign unit forms';
$string['otasignconnector:signascommander'] = 'Sign OTA Sign forms as commander';
$string['otasignconnector:configure'] = 'Configure OTA Sign Connector';
