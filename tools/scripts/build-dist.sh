#!/usr/bin/env bash
# Cross-builds, stages and signs the kbase release tarball and installer.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
repo_root="$(cd "${script_dir}/../.." && pwd)" || exit 1
cd "${repo_root}"

# The release signing key, by fingerprint: a uid can match more than one key.
SIGNING_KEY="5E19B09B9B4CD771F938734C8418EF9F1BE16E1E"

# One record per release target: os, arch, staged binary name, the platform
# name the installer resolves it to, and the uname pair that installer accepts
# it under. Every place that spells a target reads this table.
TARGETS=(
  "darwin,arm64,kbase-darwin-arm64,darwin-arm64,Darwin:arm64"
  "linux,amd64,kbase-linux-amd64,linux-amd64,Linux:x86_64|Linux:amd64"
  "windows,amd64,kbase-windows-amd64.exe,windows-amd64,MINGW64*:x86_64|MINGW64*:amd64|MSYS*:x86_64|MSYS*:amd64"
)

# dist-only build trim: -trimpath drops local filesystem paths from the
# binary; -s -w strips symbol table + DWARF (panic traces keep function
# names); -tags kbasedist compiles out the development diagnostics
# (`score --replay`). Local `just build` stays fat for debugging and keeps them.
function set_defaults() {
  BIN_DIR="${BIN_DIR:-bin}"
  DIST_DIR="${DIST_DIR:-dist}"
  RELEASE_FLAGS=(-trimpath -tags kbasedist "-ldflags=-s -w")
}

function build_targets() {
  mkdir -p "${BIN_DIR}"
  command rm -f "${BIN_DIR}"/kbase*
  local record os_name arch bin
  for record in "${TARGETS[@]}"; do
    IFS=, read -r os_name arch bin _ _ <<< "${record}"
    GOOS="${os_name}" GOARCH="${arch}" go build "${RELEASE_FLAGS[@]}" -o "${BIN_DIR}/${bin}" ./cmd
  done
}

function resolve_release_names() {
  ver="v$(go run ./cmd --version)"
  if [[ ! "${ver}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    printf 'dist: check: usage: version %q is not v<major>.<minor>.<patch>; nothing was staged or signed.\n' "${ver}" >&2
    exit 1
  fi
  ver_dir="kbase-${ver}"
  tgz="${ver_dir}.tgz"
  tgz_asc="${tgz}.asc"
  inst="install-${ver_dir}.sh"
  inst_asc="${inst}.asc"
}

# With TAG=1 the build ends by tagging the commit it built, so a tag that
# exists, or a tree that differs from HEAD, is refused before anything is built.
function check_tag_ready() {
  [[ "${TAG:-}" == "1" ]] || return 0
  if git rev-parse -q --verify "refs/tags/${ver}" > /dev/null; then
    printf 'dist: check: usage: tag %s already exists; nothing was staged or signed.\n' "${ver}" >&2
    exit 1
  fi
  if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
    printf 'dist: check: usage: tracked files have uncommitted changes, so tag %s would name a commit that did not build the release; nothing was staged or signed.\n' "${ver}" >&2
    exit 1
  fi
}

function tag_release() {
  [[ "${TAG:-}" == "1" ]] || return 0
  git tag "${ver}"
}

function stage_release() {
  command rm -rf "${DIST_DIR}/${ver_dir}" "${DIST_DIR}/${tgz}" "${DIST_DIR}/${tgz_asc}" \
    "${DIST_DIR}/${inst}" "${DIST_DIR}/${inst_asc}"
  mkdir -p "${DIST_DIR}/${ver_dir}"
  cp "${BIN_DIR}"/kbase-*-* "${DIST_DIR}/${ver_dir}/"
  cp USER_CONVENTIONS.md "${DIST_DIR}/${ver_dir}/CONVENTIONS.md"
  cp README.md LICENSE "${DIST_DIR}/${ver_dir}/"
}

function sign_staged() {
  local record bin
  for record in "${TARGETS[@]}"; do
    IFS=, read -r _ _ bin _ _ <<< "${record}"
    gpg --yes --local-user "${SIGNING_KEY}" --detach-sign --armor "${DIST_DIR}/${ver_dir}/${bin}"
  done
}

function make_tarball() {
  tar -czf "${DIST_DIR}/${tgz}" -C "${DIST_DIR}" "${ver_dir}"
}

function sign_tarball() {
  gpg --yes --local-user "${SIGNING_KEY}" --detach-sign --armor "${DIST_DIR}/${tgz}"
}

# The installer's own text that names a target: one line per TARGETS record.
function installer_platform_arms() {
  local record plat uname_case
  for record in "${TARGETS[@]}"; do
    IFS=, read -r _ _ _ plat uname_case <<< "${record}"
    printf '  %s) plat="%s" ;;\n' "${uname_case}" "${plat}"
  done
}

function installer_binary_verifications() {
  local record bin
  for record in "${TARGETS[@]}"; do
    IFS=, read -r _ _ bin _ _ <<< "${record}"
    printf 'gpg --assert-signer "${fpr}" --verify "./%s/%s.asc" "./%s/%s"\n' "${ver_dir}" "${bin}" "${ver_dir}" "${bin}"
  done
}

function write_installer() {
  cat > "${DIST_DIR}/${inst}" << EOF
#!/usr/bin/env bash
set -euo pipefail
os="\$(uname -s)"
mach="\$(uname -m)"
case "\${os}:\${mach}" in
$(installer_platform_arms)
  *)
    printf 'kbase: unsupported platform: uname -s reports %s, uname -m reports %s. kbase ships arm64 for macOS and amd64 for Linux and Windows.\n' "\${os}" "\${mach}" 1>&2
    exit 1
    ;;
esac
command -v gpg > /dev/null || { echo "gpg not installed or not in PATH."; exit 1; }
key_url="https://bennherrera.dev/wkd"
key_info=\$(gpg --auto-key-locate wkd --locate-external-keys benn@bennherrera.dev)
[[ -f "${inst_asc}" ]] || curl --fail --silent --show-error --location --proto '=https' --max-time 60 -O "https://github.com/benn-herrera/kbase/releases/download/${ver}/${inst_asc}"
otrust=\$(awk '/^uid/ { print \$2; exit(0); }' <<< "\${key_info}")
fpr=\$(awk '/^pub/ {Y=1; next;} {if (Y != 1) next; print \$1; exit(0);}' <<< "\${key_info}")
if [[ "\${otrust}" != "[ultimate]" ]]; then
  if [[ ! -t 0 ]]; then
    printf 'kbase: stdin is not a terminal, so the key cannot be trusted interactively. Nothing was verified and nothing was installed.\nEstablish the trust by hand, then rerun this script:\n' 1>&2
    exit 1
  fi
  printf '\nkbase releases are signed by benn@bennherrera.dev\n\n%s\n\n%s\n\nNeither this script nor the download is trusted until you answer.\nTrust this key for kbase releases?\n  y: record ultimate ownertrust for the fingerprint above; later installs with this key ask nothing\n  o: record nothing and verify now; the keyring stays untouched and gpg flags each signature as untrusted\n  anything else: abort, installing nothing\nChoice [y/o/N]: ' "\${key_url}" "\${key_info}" 1>&2
  answer=""
  read -r answer || answer=""
  case "\${answer}" in
    y|Y)
      printf '%s:6:\n' "\${fpr}" | gpg --import-ownertrust
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
gpg --assert-signer "\${fpr}" --verify "${inst_asc}" "\${0}"
[[ -f "${tgz_asc}" ]] || curl --fail --silent --show-error --location --proto '=https' --max-time 600 -O "https://github.com/benn-herrera/kbase/releases/download/${ver}/${tgz_asc}"
[[ -f "${tgz}" ]] || curl --fail --silent --show-error --location --proto '=https' --max-time 600 -O "https://github.com/benn-herrera/kbase/releases/download/${ver}/${tgz}"
gpg --assert-signer "\${fpr}" --verify "${tgz_asc}" "${tgz}"
[[ -d "${ver_dir}" ]] || tar xzf "${tgz}"

$(installer_binary_verifications)

case "\${plat}" in
  darwin-arm64)
    command cp -vf "./${ver_dir}/kbase-darwin-arm64" "./${ver_dir}/kbase"
    command cp -vf "./${ver_dir}/kbase-darwin-arm64.asc" "./${ver_dir}/kbase.asc"
    xattr -d com.apple.quarantine "./${ver_dir}/kbase" 2> /dev/null || true
    ;;
  linux-amd64)
    command cp -vf "./${ver_dir}/kbase-linux-amd64" "./${ver_dir}/kbase"
    command cp -vf "./${ver_dir}/kbase-linux-amd64.asc" "./${ver_dir}/kbase.asc"
    ;;
  windows-amd64)
    command cp -vf "./${ver_dir}/kbase-windows-amd64.exe" "./${ver_dir}/kbase.exe"
    command cp -vf "./${ver_dir}/kbase-windows-amd64.exe.asc" "./${ver_dir}/kbase.exe.asc"
    ;;
esac
EOF
}

function sign_installer() {
  gpg --yes --local-user "${SIGNING_KEY}" --detach-sign --armor "${DIST_DIR}/${inst}"
  chmod 755 "${DIST_DIR}/${inst}"
}

function report() {
  printf 'dist: %s %s %s  %s\n' \
    "${DIST_DIR}/${tgz}" "${DIST_DIR}/${tgz_asc}" "${DIST_DIR}/${inst}" "${DIST_DIR}/${inst_asc}"
}

function main() {
  set_defaults
  resolve_release_names
  check_tag_ready
  build_targets
  stage_release
  sign_staged
  make_tarball
  sign_tarball
  write_installer
  sign_installer
  report
  tag_release
}

main
