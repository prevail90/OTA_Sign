<?php
// This file is part of Moodle - http://moodle.org/

defined('MOODLE_INTERNAL') || die();

function local_otasignconnector_build_launch_payload(stdClass $user): array {
    $now = time();
    $uicfield = get_config('local_otasignconnector', 'uic_profile_field') ?: 'uic';
    $dodidfield = get_config('local_otasignconnector', 'dodid_profile_field') ?: 'dodid';
    $rankfield = get_config('local_otasignconnector', 'rank_profile_field') ?: 'rank';
    $armyemailfield = get_config('local_otasignconnector', 'army_email_profile_field') ?: 'armyemail';
    $armyemail = local_otasignconnector_army_email($user, $armyemailfield);

    $payload = [
        'moodle_user_id' => (string)$user->id,
        'full_name' => fullname($user),
        'first_name' => trim((string)($user->firstname ?? '')),
        'last_name' => trim((string)($user->lastname ?? '')),
        'middle_initial' => local_otasignconnector_middle_initial($user),
        'email' => (string)$user->email,
        'army_email' => $armyemail,
        'dod_id' => local_otasignconnector_profile_value($user, $dodidfield),
        'rank' => local_otasignconnector_profile_value($user, $rankfield),
        'uic' => local_otasignconnector_profile_value($user, $uicfield),
        'roles' => local_otasignconnector_user_roles($user),
        'capabilities' => local_otasignconnector_user_capabilities($user),
        'issued_at' => gmdate('c', $now),
        'expires_at' => gmdate('c', $now + 300),
    ];

    return $payload;
}

function local_otasignconnector_middle_initial(stdClass $user): string {
    $middlename = trim((string)($user->middlename ?? ''));
    if ($middlename === '') {
        return '';
    }

    return strtoupper(substr($middlename, 0, 1));
}

function local_otasignconnector_sign_payload(array $payload, string $secret): string {
    $json = json_encode($payload, JSON_UNESCAPED_SLASHES);
    $encodedpayload = local_otasignconnector_base64url_encode($json);
    $signature = hash_hmac('sha256', $encodedpayload, $secret, true);

    return $encodedpayload . '.' . local_otasignconnector_base64url_encode($signature);
}

function local_otasignconnector_profile_value(stdClass $user, string $shortname): string {
    global $DB;

    if ($shortname === '') {
        return '';
    }

    $sql = "SELECT d.data
              FROM {user_info_data} d
              JOIN {user_info_field} f ON f.id = d.fieldid
             WHERE d.userid = :userid
               AND f.shortname = :shortname";

    $value = $DB->get_field_sql($sql, [
        'userid' => $user->id,
        'shortname' => $shortname,
    ]);

    if ($value === false) {
        return '';
    }

    return trim(preg_replace('/\s+/', ' ', strip_tags((string)$value)));
}

function local_otasignconnector_army_email(stdClass $user, string $shortname): string {
    $customemail = local_otasignconnector_profile_value($user, $shortname);
    if (local_otasignconnector_is_army_email($customemail)) {
        return strtolower($customemail);
    }

    $profileemail = trim((string)($user->email ?? ''));
    if (local_otasignconnector_is_army_email($profileemail)) {
        return strtolower($profileemail);
    }

    return '';
}

function local_otasignconnector_is_army_email(string $email): bool {
    $email = strtolower(trim($email));
    return $email !== '' && preg_match('/^[^@\s]+@army\.mil$/', $email) === 1;
}

function local_otasignconnector_user_roles(stdClass $user): array {
    $context = context_system::instance();
    $roles = get_user_roles($context, $user->id, false);
    $names = [];

    foreach ($roles as $role) {
        $names[] = $role->shortname;
    }

    return array_values(array_unique($names));
}

function local_otasignconnector_user_capabilities(stdClass $user): array {
    $context = context_system::instance();
    $capabilities = [];

    $map = [
        'local/otasignconnector:viewown' => 'viewown',
        'local/otasignconnector:viewunit' => 'viewunit',
        'local/otasignconnector:signascommander' => 'signascommander',
        'local/otasignconnector:configure' => 'configure',
    ];

    foreach ($map as $moodlecapability => $portalcapability) {
        if (has_capability($moodlecapability, $context, $user)) {
            $capabilities[] = $portalcapability;
        }
    }

    return $capabilities;
}

function local_otasignconnector_base64url_encode(string $value): string {
    return rtrim(strtr(base64_encode($value), '+/', '-_'), '=');
}
