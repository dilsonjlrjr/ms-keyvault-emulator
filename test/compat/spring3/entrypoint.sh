#!/bin/sh
set -e

CA_FILE="/certs/ca.pem"
CACERTS="/opt/java/openjdk/lib/security/cacerts"

if [ -f "$CA_FILE" ]; then
    echo "[entrypoint] Importing kvemu CA into JRE trust store..."
    keytool -importcert -noprompt \
        -alias kvemu-ca \
        -file "$CA_FILE" \
        -keystore "$CACERTS" \
        -storepass changeit 2>&1 | grep -v "already exists" || true
    echo "[entrypoint] CA imported."
else
    echo "[entrypoint] WARNING: $CA_FILE not found, HTTPS to kvemu may fail."
fi

if [ -n "${AZURE_AUTHORITY_HOST}" ]; then
    AUTH_JAVA_OPTS="-Dazure.identity.azure-authority-host=${AZURE_AUTHORITY_HOST}"
else
    AUTH_JAVA_OPTS=""
fi
echo "[entrypoint] AZURE_AUTHORITY_HOST=${AZURE_AUTHORITY_HOST}"
echo "[entrypoint] Starting Spring Boot 3 app..."
exec java ${AUTH_JAVA_OPTS} -jar /app/app.jar "$@"
