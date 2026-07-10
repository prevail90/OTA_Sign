#!/bin/sh
set -eu

cat > /usr/share/nginx/html/config.js <<EOF
window.__OTASIGN_CONFIG__ = {
  apiBaseUrl: "${OTASIGN_API_BASE_URL:-http://localhost:8080}",
  moodleLoginUrl: "${OTASIGN_MOODLE_LOGIN_URL:-http://localhost:8081/login/index.php}",
  moodleLaunchUrl: "${OTASIGN_MOODLE_LAUNCH_URL:-http://localhost:8081/local/otasignconnector/launch.php}"
};
EOF
