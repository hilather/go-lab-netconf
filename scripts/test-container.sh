#!/usr/bin/env bash
# Container contract. Requires Docker. Fail closed if the daemon is missing.
# Default path: --netconf-listen=:1830 --restconf-listen=:8303 and cap_drop ALL.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IMAGE="${LABNETCONF_TEST_IMAGE:-ghcr.io/hilather/labnetconf:test}"
NAME="labnetconf-container-test-$$"
COMPOSE="${ROOT}/examples/compose.smoke.yaml"
TOKEN="${ROOT}/testdata/container/token"
CONFIG="${ROOT}/testdata/container/config.yaml"
HOSTKEY="${ROOT}/testdata/keys/labnetconf-hostkey"
ALICE_PW="${ROOT}/testdata/keys/alice.password"

if ! command -v docker >/dev/null 2>&1; then
	echo "docker is required for make test-container" >&2
	exit 1
fi
if ! docker info >/dev/null 2>&1; then
	echo "docker daemon is not available for make test-container" >&2
	exit 1
fi
if ! command -v curl >/dev/null 2>&1; then
	echo "curl is required for make test-container" >&2
	exit 1
fi
if [ ! -f "${TOKEN}" ] || [ ! -f "${CONFIG}" ] || [ ! -f "${HOSTKEY}" ] || [ ! -f "${ALICE_PW}" ]; then
	echo "missing testdata/container/{token,config.yaml} or testdata/keys/{labnetconf-hostkey,alice.password}" >&2
	exit 1
fi

cleanup() {
	docker rm -f "${NAME}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "building ${IMAGE}"
docker build -t "${IMAGE}" "${ROOT}"

inspect_user="$(docker image inspect --format '{{.Config.User}}' "${IMAGE}")"
if [ "${inspect_user}" != "65532:65532" ]; then
	echo "image User=${inspect_user}, want 65532:65532" >&2
	exit 1
fi

licenses="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.licenses"}}' "${IMAGE}")"
if [ "${licenses}" != "Apache-2.0" ]; then
	echo "image license label=${licenses}, want Apache-2.0" >&2
	exit 1
fi

hc="$(docker image inspect --format '{{json .Config.Healthcheck.Test}}' "${IMAGE}")"
case "${hc}" in
*CMD-SHELL*)
	echo "image HEALTHCHECK=${hc} must be exec form, not shell" >&2
	exit 1
	;;
esac
case "${hc}" in
'["CMD",'*)
	;;
*)
	echo "image HEALTHCHECK=${hc}, want JSON array starting with CMD" >&2
	exit 1
	;;
esac
case "${hc}" in
*'/v1/health/ready'*)
	;;
*)
	echo "image HEALTHCHECK=${hc}, want /v1/health/ready" >&2
	exit 1
	;;
esac
case "${hc}" in
*healthcheck*)
	;;
*)
	echo "image HEALTHCHECK=${hc}, want exec-form labnetconf healthcheck" >&2
	exit 1
	;;
esac

if docker compose version >/dev/null 2>&1; then
	docker compose -f "${COMPOSE}" config >/dev/null
else
	echo "docker compose plugin not available; compose file parse skipped" >&2
fi

docker run -d --name "${NAME}" \
	--read-only \
	--cap-drop=ALL \
	--security-opt=no-new-privileges:true \
	--tmpfs /tmp:rw,noexec,nosuid,size=16m \
	-v "${CONFIG}:/etc/labnetconf/config.yaml:ro" \
	-v "${TOKEN}:/etc/labnetconf/token:ro" \
	-v "${HOSTKEY}:/etc/labnetconf/hostkey:ro" \
	-v "${ALICE_PW}:/etc/labnetconf/alice.password:ro" \
	-p 127.0.0.1:0:1830/tcp \
	-p 127.0.0.1:0:8303/tcp \
	-p 127.0.0.1:0:8088/tcp \
	"${IMAGE}" \
	serve --config=/etc/labnetconf/config.yaml \
		--netconf-listen=:1830 \
		--restconf-listen=:8303 \
		--management-listen=:8088

if [ "$(docker inspect --format '{{.State.Running}}' "${NAME}")" != "true" ]; then
	echo "container is not running" >&2
	docker inspect --format 'status={{.State.Status}} exit={{.State.ExitCode}} error={{.State.Error}}' "${NAME}" >&2 || true
	docker logs "${NAME}" >&2 || true
	exit 1
fi

readonly_root="$(docker inspect --format '{{.HostConfig.ReadonlyRootfs}}' "${NAME}")"
if [ "${readonly_root}" != "true" ]; then
	echo "HostConfig.ReadonlyRootfs=${readonly_root}, want true" >&2
	exit 1
fi

mgmt_port="$(docker port "${NAME}" 8088/tcp | head -n1 | awk -F: '{print $NF}')"
rc_port="$(docker port "${NAME}" 8303/tcp | head -n1 | awk -F: '{print $NF}')"
nc_port="$(docker port "${NAME}" 1830/tcp | head -n1 | awk -F: '{print $NF}')"
if [ -z "${mgmt_port}" ] || [ -z "${rc_port}" ] || [ -z "${nc_port}" ]; then
	echo "published ports missing mgmt=${mgmt_port} restconf=${rc_port} netconf=${nc_port}" >&2
	docker inspect --format '{{json .HostConfig.PortBindings}}' "${NAME}" >&2 || true
	docker logs "${NAME}" >&2 || true
	exit 1
fi

ok=0
for _ in $(seq 1 40); do
	if curl -fsS "http://127.0.0.1:${mgmt_port}/v1/health/ready" >/dev/null 2>&1; then
		ok=1
		break
	fi
	sleep 0.25
done
if [ "${ok}" -ne 1 ]; then
	echo "management ready check failed on 127.0.0.1:${mgmt_port}" >&2
	docker inspect --format 'status={{.State.Status}} exit={{.State.ExitCode}}' "${NAME}" >&2 || true
	docker logs "${NAME}" >&2 || true
	exit 1
fi

ui_code="$(curl -sS -D /tmp/labnetconf-ui.hdr -o /tmp/labnetconf-ui.body -w '%{http_code}' "http://127.0.0.1:${mgmt_port}/" || true)"
if [ "${ui_code}" != "404" ]; then
	echo "GET / status=${ui_code}, want 404 (ui.enabled: false)" >&2
	cat /tmp/labnetconf-ui.hdr >&2 || true
	cat /tmp/labnetconf-ui.body >&2 || true
	exit 1
fi
if ! grep -qi 'content-type:.*application/problem+json' /tmp/labnetconf-ui.hdr; then
	echo "GET / content-type is not application/problem+json" >&2
	cat /tmp/labnetconf-ui.hdr >&2 || true
	exit 1
fi
if ! curl -fsS "http://127.0.0.1:${mgmt_port}/v1/health/ready" >/dev/null; then
	echo "GET /v1/health/ready failed after SPA-disabled 404" >&2
	exit 1
fi

if ! docker exec "${NAME}" /labnetconf version >/dev/null; then
	echo "non-root exec of /labnetconf version failed" >&2
	exit 1
fi
if ! docker exec "${NAME}" /labnetconf healthcheck --url=http://127.0.0.1:8088/v1/health/ready >/dev/null; then
	echo "in-container HTTP ready healthcheck failed" >&2
	exit 1
fi
if docker exec "${NAME}" /bin/sh -c true >/dev/null 2>&1; then
	echo "image has a shell at /bin/sh" >&2
	exit 1
fi

hostname_json="$(curl -fsS -u 'alice:alice-lab-password' -H 'Accept: application/yang-data+json' \
	"http://127.0.0.1:${rc_port}/restconf/data/ietf-system:system/hostname")"
if ! printf '%s\n' "${hostname_json}" | grep -q 'lab-rtr-a'; then
	echo "RESTCONF GET hostname missing lab-rtr-a: ${hostname_json}" >&2
	docker logs "${NAME}" >&2 || true
	exit 1
fi

if ! python3 -c "import socket; s=socket.create_connection(('127.0.0.1', int('${nc_port}')), 2); s.close()" 2>/dev/null; then
	echo "NETCONF :1830 (host ${nc_port}) is not accepting TCP" >&2
	docker logs "${NAME}" >&2 || true
	exit 1
fi

echo "container contract ok image=${IMAGE}"
