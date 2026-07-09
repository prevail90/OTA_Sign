<?php
// This file is part of Moodle - http://moodle.org/

defined('MOODLE_INTERNAL') || die();

if ($hassiteconfig) {
    $settings = new admin_settingpage(
        'local_otasignconnector',
        get_string('pluginname', 'local_otasignconnector')
    );

    $settings->add(new admin_setting_configtext(
        'local_otasignconnector/launch_url',
        get_string('launch_url', 'local_otasignconnector'),
        get_string('launch_url_desc', 'local_otasignconnector'),
        'http://localhost:8080/launch',
        PARAM_URL
    ));

    $settings->add(new admin_setting_configpasswordunmask(
        'local_otasignconnector/signing_secret',
        get_string('signing_secret', 'local_otasignconnector'),
        get_string('signing_secret_desc', 'local_otasignconnector'),
        ''
    ));

    $settings->add(new admin_setting_configtext(
        'local_otasignconnector/uic_profile_field',
        get_string('uic_profile_field', 'local_otasignconnector'),
        get_string('uic_profile_field_desc', 'local_otasignconnector'),
        'uic',
        PARAM_ALPHANUMEXT
    ));

    $settings->add(new admin_setting_configtext(
        'local_otasignconnector/dodid_profile_field',
        get_string('dodid_profile_field', 'local_otasignconnector'),
        get_string('dodid_profile_field_desc', 'local_otasignconnector'),
        'dodid',
        PARAM_ALPHANUMEXT
    ));

    $settings->add(new admin_setting_configtext(
        'local_otasignconnector/rank_profile_field',
        get_string('rank_profile_field', 'local_otasignconnector'),
        get_string('rank_profile_field_desc', 'local_otasignconnector'),
        'rank',
        PARAM_ALPHANUMEXT
    ));

    $settings->add(new admin_setting_configtext(
        'local_otasignconnector/paygrade_profile_field',
        get_string('paygrade_profile_field', 'local_otasignconnector'),
        get_string('paygrade_profile_field_desc', 'local_otasignconnector'),
        'paygrade',
        PARAM_ALPHANUMEXT
    ));

    $ADMIN->add('localplugins', $settings);
}
