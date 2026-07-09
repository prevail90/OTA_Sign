<?php
// This file is part of Moodle - http://moodle.org/

require_once(__DIR__ . '/../../config.php');
require_once($CFG->libdir . '/filelib.php');
require_once(__DIR__ . '/locallib.php');

require_login();

$launchurl = get_config('local_otasignconnector', 'launch_url');
$secret = get_config('local_otasignconnector', 'signing_secret');

if (empty($launchurl) || empty($secret)) {
    throw new moodle_exception('missingconfig', 'local_otasignconnector');
}

$payload = local_otasignconnector_build_launch_payload($USER);
$token = local_otasignconnector_sign_payload($payload, $secret);

$redirect = new moodle_url($launchurl, ['token' => $token]);
redirect($redirect);
