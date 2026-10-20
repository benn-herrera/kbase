#!/usr/bin/env bash
set -euo pipefail
os="$(uname -s)"
mach="$(uname -m)"
case "${os}:${mach}" in
  Darwin:arm64) plat="darwin-arm64" ;;
  Linux:x86_64|Linux:amd64) plat="linux-amd64" ;;
  MINGW64*:x86_64|MINGW64*:amd64|MSYS*:x86_64|MSYS*:amd64) plat="windows-amd64" ;;
  *)
    printf 'kbase: unsupported platform: uname -s reports %s, uname -m reports %s. kbase ships arm64 for macOS and amd64 for Linux and Windows.\n' "${os}" "${mach}" 1>&2
    exit 1
    ;;
esac
command -v gpg > /dev/null || { echo "gpg not installed or not in PATH."; exit 1; }
key_url="https://bennherrera.dev/.well-known/openpgpkey/hu/dpqu1e7ew9iu17qy81b7oqfb8yq11or4"
policy_url="https://bennherrera.dev/.well-known/openpgpkey/policy"
gpg --auto-key-locate wkd --locate-keys benn@bennherrera.dev
[[ -f "install-kbase-v2.0.0.sh.asc" ]] || curl --fail --silent --show-error --location --proto '=https' --max-time 60 -O "https://github.com/benn-herrera/kbase/releases/download/v2.0.0/install-kbase-v2.0.0.sh.asc"
packets="$(gpg --batch --list-packets "install-kbase-v2.0.0.sh.asc")"
fpr="$(printf '%s\n' "${packets}" | awk '/issuer fpr v4 / { sub(/.*issuer fpr v4 /, ""); sub(/\).*/, ""); print; exit }')"
lookup="${fpr}"
[[ -n "${lookup}" ]] || lookup="$(printf '%s\n' "${packets}" | awk '/keyid / && /:signature packet:/ { sub(/.*keyid /, ""); print; exit }')"
[[ -n "${lookup}" ]] || {
  printf 'kbase: cannot say who signed this script: its signature names no key. Nothing was verified and nothing was installed.\n' 1>&2
  exit 1
}
if ! listing="$(gpg --batch --with-colons --list-keys "${lookup}")"; then
  printf 'kbase: this script is signed by key %s, which is not in the keyring. The kbase signing key is published at %s. Nothing was verified and nothing was installed.\n' "${lookup}" "${key_url}" 1>&2
  exit 1
fi
[[ -n "${fpr}" ]] || fpr="$(printf '%s\n' "${listing}" | awk -F: '$1 == "fpr" { print $10; exit }')"
uid="$(printf '%s\n' "${listing}" | awk -F: '$1 == "uid" { print $10; exit }')"
otrust="$(printf '%s\n' "${listing}" | awk -F: '$1 == "pub" { print $9; exit }')"
[[ -n "${fpr}" && -n "${uid}" ]] || {
  printf 'kbase: gpg listed no fingerprint or user ID for signing key %s, so nothing can be trusted. Nothing was verified and nothing was installed.\n' "${lookup}" 1>&2
  exit 1
}
if [[ "${otrust}" != "u" ]]; then
  if [[ ! -t 0 ]]; then
    printf 'kbase: stdin is not a terminal, so the key cannot be trusted interactively. Nothing was verified and nothing was installed.\nEstablish the trust by hand, then rerun this script:\n' 1>&2
    printf '  curl -fsSL %s | gpg --import\n' "${key_url}" 1>&2
    printf '  echo %s:6: | gpg --import-ownertrust\n' "${fpr}" 1>&2
    exit 1
  fi
  printf 'kbase releases are signed by:\n  uid:         %s\n  fingerprint: %s\n  key:         %s\n  policy:      %s\nHolding a copy of the key and of the policy is how you check both for yourself.\nNeither this script nor the download is trusted until you answer.\nTrust this key for kbase releases?\n  y: record ultimate ownertrust for the fingerprint above; later installs with this key ask nothing\n  o: record nothing and verify now; the keyring stays untouched and gpg flags each signature as untrusted\n  anything else: abort, installing nothing\nChoice [y/o/N]: ' "${uid}" "${fpr}" "${key_url}" "${policy_url}" 1>&2
  answer=""
  read -r answer || answer=""
  case "${answer}" in
    y|Y)
      printf '%s:6:\n' "${fpr}" | gpg --batch --import-ownertrust
      ;;
    o|O)
      ;;
    *)
      printf 'kbase: trust not confirmed. Nothing was verified and nothing was installed.\n' 1>&2
      exit 1
      ;;
  esac
fi
printf "If gpg check fails for the tgz or any binary DO NOT USE THIS DOWNLOAD!\n\n"
gpg --verify "install-kbase-v2.0.0.sh.asc" "${0}"
[[ -f "kbase-v2.0.0.tgz.asc" ]] || curl --fail --silent --show-error --location --proto '=https' --max-time 600 -O "https://github.com/benn-herrera/kbase/releases/download/v2.0.0/kbase-v2.0.0.tgz.asc"
[[ -f "kbase-v2.0.0.tgz" ]] || curl --fail --silent --show-error --location --proto '=https' --max-time 600 -O "https://github.com/benn-herrera/kbase/releases/download/v2.0.0/kbase-v2.0.0.tgz"
gpg --verify "kbase-v2.0.0.tgz.asc" "kbase-v2.0.0.tgz"
[[ -d "kbase-v2.0.0" ]] || tar xzf "kbase-v2.0.0.tgz"

gpg --verify "./kbase-v2.0.0/kbase-darwin-arm64.asc" "./kbase-v2.0.0/kbase-darwin-arm64"
gpg --verify "./kbase-v2.0.0/kbase-linux-amd64.asc" "./kbase-v2.0.0/kbase-linux-amd64"
gpg --verify "./kbase-v2.0.0/kbase-windows-amd64.exe.asc" "./kbase-v2.0.0/kbase-windows-amd64.exe"

case "${plat}" in
  darwin-arm64)
    command cp -vf "./kbase-v2.0.0/kbase-darwin-arm64" "./kbase-v2.0.0/kbase"
    xattr -d com.apple.quarantine "./kbase-v2.0.0/kbase" 2> /dev/null || true
    ;;
  linux-amd64)
    command cp -vf "./kbase-v2.0.0/kbase-linux-amd64" "./kbase-v2.0.0/kbase"
    ;;
  windows-amd64)
    command cp -vf "./kbase-v2.0.0/kbase-windows-amd64.exe" "./kbase-v2.0.0/kbase.exe"
    ;;
esac
