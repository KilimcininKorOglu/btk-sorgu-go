#!/usr/bin/env bash
#
# Automated installer for btk-sorgu-go on Linux servers.
# Downloads the latest (or a given) release, installs it under /opt, configures
# a systemd service, sets up an nginx reverse proxy, opens the firewall (80/443),
# and optionally enables Let's Encrypt TLS.
#
# Usage:
#   sudo ./install.sh [VERSION]
#   curl -fsSL <raw-url>/install.sh | sudo bash
#
# Environment variables (optional, enable non-interactive runs):
#   VERSION     Release tag to install (e.g. v1.0.3). Empty = latest.
#   DOMAIN      Domain name for nginx server_name and TLS.
#   EMAIL       Contact email for Let's Encrypt.
#   ENABLE_SSL  "1" to request a Let's Encrypt certificate (requires DOMAIN).

set -euo pipefail

# --- Constants -------------------------------------------------------------
REPO="KilimcininKorOglu/btk-sorgu-go"
INSTALL_DIR="/opt/btk-sorgu-go"
SERVICE_NAME="btk-sorgu"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
APP_PORT="8080"

# --- Logging helpers -------------------------------------------------------
log()  { printf '[INFO] %s\n' "$*"; }
warn() { printf '[WARN] %s\n' "$*" >&2; }
ok()   { printf '[OK] %s\n' "$*"; }
die()  { printf '[WARN] %s\n' "$*" >&2; exit 1; }

# --- Pre-flight: require root ---------------------------------------------
if [[ "${EUID}" -ne 0 ]]; then
	die "Bu script root yetkisi gerektirir. 'sudo' ile çalıştırın."
fi

# --- Inputs (arg or env) ---------------------------------------------------
VERSION="${1:-${VERSION:-}}"
DOMAIN="${DOMAIN:-}"
EMAIL="${EMAIL:-}"
ENABLE_SSL="${ENABLE_SSL:-}"

# detectDistro sets PKG_MANAGER, SERVICE_USER, FIREWALL from /etc/os-release
detectDistro() {
	[[ -r /etc/os-release ]] || die "/etc/os-release bulunamadı, dağıtım algılanamıyor."
	# shellcheck disable=SC1091
	. /etc/os-release
	local id="${ID:-}" like="${ID_LIKE:-}"
	case "${id} ${like}" in
	*debian* | *ubuntu*)
		PKG_MANAGER="apt-get"
		SERVICE_USER="www-data"
		FIREWALL="ufw"
		;;
	*rhel* | *centos* | *fedora* | *rocky* | *almalinux*)
		if command -v dnf >/dev/null 2>&1; then PKG_MANAGER="dnf"; else PKG_MANAGER="yum"; fi
		SERVICE_USER="nobody"
		FIREWALL="firewalld"
		;;
	*)
		die "Desteklenmeyen dağıtım: ID=${id} ID_LIKE=${like}"
		;;
	esac
	log "Dağıtım: ${id} (paket: ${PKG_MANAGER}, kullanıcı: ${SERVICE_USER}, firewall: ${FIREWALL})"
}

# detectArch sets ARCH (amd64|arm64) from uname -m
detectArch() {
	local m
	m="$(uname -m)"
	case "${m}" in
	x86_64 | amd64) ARCH="amd64" ;;
	aarch64 | arm64) ARCH="arm64" ;;
	*) die "Desteklenmeyen mimari: ${m}" ;;
	esac
	log "Mimari: ${ARCH}"
}

# ensureCommand installs the package providing a command when it is missing
ensureCommand() {
	local cmd="$1" pkg="${2:-$1}"
	command -v "${cmd}" >/dev/null 2>&1 && return 0
	log "'${cmd}' bulunamadı, kuruluyor (${pkg})..."
	if [[ "${PKG_MANAGER}" == "apt-get" ]]; then
		apt-get update -qq
		DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "${pkg}"
	else
		"${PKG_MANAGER}" install -y -q "${pkg}"
	fi
}

# resolveVersion echoes the release tag to install (latest when VERSION empty)
resolveVersion() {
	if [[ -n "${VERSION}" ]]; then
		printf '%s' "${VERSION}"
		return 0
	fi
	local tag
	tag="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
		grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name" *: *"([^"]+)".*/\1/')"
	[[ -n "${tag}" ]] || die "En son sürüm GitHub API'den alınamadı."
	printf '%s' "${tag}"
}

# downloadAndInstall downloads the release archive, verifies its checksum,
# and installs the binary plus .env under INSTALL_DIR
downloadAndInstall() {
	local tag="$1"
	local archive="btk-sorgu-go_linux_${ARCH}.tar.gz"
	local base="https://github.com/${REPO}/releases/download/${tag}"
	local tmp
	tmp="$(mktemp -d)"
	# Clean up the temp dir on exit
	trap 'rm -rf "${tmp}"' RETURN

	log "İndiriliyor: ${tag} (${archive})"
	curl -fsSL -o "${tmp}/${archive}" "${base}/${archive}" || die "Arşiv indirilemedi: ${base}/${archive}"

	# Verify SHA256 against the release checksums file
	if curl -fsSL -o "${tmp}/checksums.txt" "${base}/checksums.txt" 2>/dev/null; then
		log "SHA256 doğrulanıyor..."
		(cd "${tmp}" && grep " ${archive}\$" checksums.txt | sha256sum -c -) ||
			die "Sağlama toplamı doğrulaması başarısız."
		ok "Sağlama toplamı doğrulandı."
	else
		warn "checksums.txt indirilemedi, sağlama toplamı doğrulaması atlandı."
	fi

	tar -xzf "${tmp}/${archive}" -C "${tmp}" || die "Arşiv açılamadı."
	[[ -f "${tmp}/btk-sorgu" ]] || die "Arşiv içinde 'btk-sorgu' binary'si bulunamadı."

	# Install binary under the arch-suffixed name the systemd unit expects
	install -d -m 0755 "${INSTALL_DIR}"
	BINARY_PATH="${INSTALL_DIR}/btk-sorgu_linux_${ARCH}"
	install -m 0755 "${tmp}/btk-sorgu" "${BINARY_PATH}"
	ok "Binary kuruldu: ${BINARY_PATH}"

	# Create .env from the example only when absent (idempotent, preserves edits)
	if [[ ! -f "${INSTALL_DIR}/.env" ]]; then
		install -m 0640 "${tmp}/.env.example" "${INSTALL_DIR}/.env"
		ok ".env oluşturuldu (varsayılan PORT=${APP_PORT}, localhost)."
	else
		log ".env zaten mevcut, korunuyor."
	fi
}

# setupService writes the systemd unit and starts the service
setupService() {
	log "systemd servisi yapılandırılıyor..."
	# RHEL-family lacks ProtectSystem=strict friendliness on old systemd; use full there
	local protectSystem="strict"
	local rwLine="ReadWritePaths=${INSTALL_DIR}"
	if [[ "${FIREWALL}" == "firewalld" ]]; then
		protectSystem="full"
		rwLine=""
	fi

	cat >"${SERVICE_FILE}" <<EOF
[Unit]
Description=BTK Engel Kontrol API
Documentation=https://github.com/${REPO}
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${BINARY_PATH}
Restart=always
RestartSec=5
EnvironmentFile=${INSTALL_DIR}/.env
NoNewPrivileges=true
ProtectSystem=${protectSystem}
ProtectHome=true
PrivateTmp=true
${rwLine}
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${SERVICE_NAME}

[Install]
WantedBy=multi-user.target
EOF

	# Ensure the service account owns the install dir (for .env readability)
	chown -R "${SERVICE_USER}:${SERVICE_USER}" "${INSTALL_DIR}"
	systemctl daemon-reload
	systemctl enable --now "${SERVICE_NAME}"
	ok "Servis başlatıldı: ${SERVICE_NAME}"
}

# setupNginx installs nginx and configures a reverse proxy to the local app
setupNginx() {
	ensureCommand nginx nginx
	local serverName="${DOMAIN:-_}"
	local confPath
	if [[ -d /etc/nginx/sites-available ]]; then
		confPath="/etc/nginx/sites-available/${SERVICE_NAME}"
	else
		confPath="/etc/nginx/conf.d/${SERVICE_NAME}.conf"
	fi

	cat >"${confPath}" <<EOF
server {
    listen 80;
    server_name ${serverName};

    location / {
        proxy_pass http://127.0.0.1:${APP_PORT};
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
EOF

	# Enable the site on Debian-style layouts
	if [[ -d /etc/nginx/sites-enabled ]]; then
		ln -sf "${confPath}" "/etc/nginx/sites-enabled/${SERVICE_NAME}"
		# Drop the default site so server_name _ works predictably
		rm -f /etc/nginx/sites-enabled/default
	fi

	nginx -t || die "nginx yapılandırması geçersiz."
	systemctl enable --now nginx
	systemctl reload nginx
	ok "nginx reverse proxy yapılandırıldı (80 -> 127.0.0.1:${APP_PORT})."
}

# setupFirewall opens 80/443 (never 8080). App port stays localhost-only.
setupFirewall() {
	if [[ "${FIREWALL}" == "ufw" ]]; then
		ensureCommand ufw ufw
		if ! ufw status | grep -q "Status: active"; then
			# Allow SSH first so enabling ufw cannot lock out the session
			ufw allow OpenSSH 2>/dev/null || ufw allow 22/tcp
			ufw --force enable
		fi
		ufw allow 80/tcp
		ufw allow 443/tcp
	else
		ensureCommand firewall-cmd firewalld
		systemctl enable --now firewalld
		firewall-cmd --permanent --add-service=http
		firewall-cmd --permanent --add-service=https
		firewall-cmd --reload
	fi
	ok "Firewall: 80/443 açıldı (8080 dışa kapalı)."
}

# maybeSetupTLS optionally obtains a Let's Encrypt certificate via certbot
maybeSetupTLS() {
	# Ask interactively only when ENABLE_SSL was not preset and we have a TTY
	if [[ -z "${ENABLE_SSL}" && -t 0 ]]; then
		read -r -p "Let's Encrypt ile HTTPS (443) kurulsun mu? [e/H]: " answer
		case "${answer}" in
		e | E | evet | yes | y | Y) ENABLE_SSL="1" ;;
		*) ENABLE_SSL="0" ;;
		esac
	fi
	[[ "${ENABLE_SSL}" == "1" ]] || {
		log "SSL adımı atlandı (HTTP/80 üzerinden çalışıyor)."
		return 0
	}

	if [[ -z "${DOMAIN}" && -t 0 ]]; then
		read -r -p "Domain adı (örn. sorgu.example.com): " DOMAIN
	fi
	[[ -n "${DOMAIN}" ]] || {
		warn "DOMAIN belirtilmedi, SSL atlanıyor."
		return 0
	}
	if [[ -z "${EMAIL}" && -t 0 ]]; then
		read -r -p "Let's Encrypt iletişim e-postası: " EMAIL
	fi

	# certbot nginx plugin package name differs across families
	if [[ "${PKG_MANAGER}" == "apt-get" ]]; then
		ensureCommand certbot certbot
		DEBIAN_FRONTEND=noninteractive apt-get install -y -qq python3-certbot-nginx
	else
		# certbot lives in EPEL on the RHEL family; enable it first
		"${PKG_MANAGER}" install -y -q epel-release || warn "epel-release kurulamadı, certbot bulunamayabilir."
		ensureCommand certbot certbot
		"${PKG_MANAGER}" install -y -q python3-certbot-nginx
	fi

	local emailArgs=(--register-unsafely-without-email)
	[[ -n "${EMAIL}" ]] && emailArgs=(-m "${EMAIL}")
	# Service and nginx are already up; a TLS failure should not abort the install.
	# Warn and keep serving over HTTP/80 instead of dying.
	if certbot --nginx -d "${DOMAIN}" "${emailArgs[@]}" --agree-tos --redirect --non-interactive; then
		ok "HTTPS etkinleştirildi: https://${DOMAIN} (otomatik yenileme aktif)."
	else
		warn "certbot ile sertifika alınamadı; kurulum HTTP/80 üzerinden çalışmaya devam ediyor."
		warn "Sorunu giderdikten sonra elle çalıştırın: certbot --nginx -d ${DOMAIN}"
	fi
}

# verifyInstall checks the service is up and answering on the health endpoint
verifyInstall() {
	systemctl is-active --quiet "${SERVICE_NAME}" || die "Servis aktif değil. 'journalctl -u ${SERVICE_NAME}' ile inceleyin."
	if curl -fsS "http://127.0.0.1:${APP_PORT}/health" >/dev/null 2>&1; then
		ok "Sağlık kontrolü başarılı: /health yanıt veriyor."
	else
		warn "Servis aktif ama /health yanıtı alınamadı, logları kontrol edin."
	fi
}

main() {
	detectDistro
	detectArch
	ensureCommand curl curl
	ensureCommand tar tar

	local tag
	tag="$(resolveVersion)"
	log "Kurulacak sürüm: ${tag}"

	downloadAndInstall "${tag}"
	setupService
	setupNginx
	setupFirewall
	maybeSetupTLS
	verifyInstall

	local scheme="http" host="${DOMAIN:-<sunucu-ip>}"
	[[ "${ENABLE_SSL}" == "1" && -n "${DOMAIN}" ]] && scheme="https"
	printf '\n'
	ok "Kurulum tamamlandı."
	log "Erişim: ${scheme}://${host}/  (servis: ${SERVICE_NAME}, port ${APP_PORT} localhost)"
	log "Yönetim: systemctl status ${SERVICE_NAME} | journalctl -u ${SERVICE_NAME} -f"
}

main "$@"
