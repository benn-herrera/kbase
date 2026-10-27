#!/usr/bin/env bash
# Proves tools/scripts/build-dist.sh: the release layout it stages, the
# installer it generates, the platforms that installer accepts, what it
# verifies before it installs anything, and what it does about the key that
# signed it. Runs against the stubs in tools/scripts/testdata/stubs, with no
# network, no signing key and no go toolchain, inside
# test-data/transient/test-dist/run/ only, beside the recipe's log. NEEDS
# expect on PATH: the trust prompt is answered over a pty, and the trust cases
# are why the suite exists.
set -euo pipefail
export LC_ALL=C

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
repo_root="$(cd "${script_dir}/../.." && pwd)" || exit 1
stubs="${script_dir}/testdata/stubs"
work="${repo_root}/test-data/transient/test-dist/run"

go_version="2.0.0"
version="v${go_version}"
ver_dir="kbase-${version}"
tgz="${ver_dir}.tgz"
tgz_asc="${tgz}.asc"
installer="install-kbase-${version}.sh"
installer_asc="${installer}.asc"

signer_fpr="9F3A1C7B5D2E8064A1B2C3D4E5F60718293A4B5C"
signer_uid="Benn Herrera <benn@bennherrera.dev>"
other_fpr="0123456789ABCDEF0123456789ABCDEF01234567"
other_uid="Other Person <other@example.org>"

bin="${work}/bin"
dist="${work}/dist"
checks="${work}/checks"
pty_script="${work}/installer.exp"
expect_bin="$(command -v expect || true)"

passed=0
failed=0

function pass() {
  printf 'PASS %s\n' "${1}"
  passed=$((passed + 1))
}

function fail() {
  printf 'FAIL %s — %s\n' "${1}" "${2}" >&2
  failed=$((failed + 1))
}

# run_case <name> <function>: the function is the case's one assertion; what it
# prints lands in its own log.
function run_case() {
  local name="${1}" fn="${2}"
  if "${fn}" > "${checks}/${name}.log" 2>&1; then
    pass "${name}"
  else
    fail "${name}" "see ${checks}/${name}.log"
  fi
}

function file_mode() {
  case "$(uname -s)" in
    Darwin|*BSD) stat -f '%Lp' "${1}" ;;
    *) stat -c '%a' "${1}" ;;
  esac
}

function write_pty_driver() {
  cat > "${pty_script}" << 'EXPECT'
log_file $env(CASE_LOG)
set timeout 60
cd $env(CASE_DIR)
spawn -noecho bash $env(INSTALLER)
set prompt {Choice \[y/o/N\]: }
expect {
    -re $prompt {
        send -- "$env(ANSWER)\r"
        exp_continue
    }
    eof {}
    timeout { puts stderr "pty: the installer neither prompted nor finished"; exit 3 }
}
catch wait result
exit [lindex $result 3]
EXPECT
}

for stub in curl git go gpg uname; do
  if [[ ! -x "${stubs}/${stub}" ]]; then
    printf 'error: stub missing or not executable: %s\n' "${stubs}/${stub}" >&2
    exit 1
  fi
done

if [[ -z "${expect_bin}" ]]; then
  printf 'error: expect is not on PATH. It answers the trust prompt over a pty, and those cases are why this suite exists; install expect and rerun.\n' >&2
  exit 1
fi

command rm -rf "${work}"
mkdir -p "${bin}" "${dist}" "${checks}" "${work}/stub"
write_pty_driver

# build_release <name> [version [NAME=value...]]: cross-build, stage and sign
# into ${bin} and ${dist} with the stubs; the exit code lands in <name>.exit.
# The NAME=value pairs join the environment after the defaults, so they win.
function build_release() {
  local name="${1}" reported="${2:-${go_version}}" rc
  shift $(($# < 2 ? $# : 2))
  set +e
  env "PATH=${stubs}:${PATH}" "STUB_STATE_DIR=${work}/stub" "STUB_GO_VERSION=${reported}" \
    "BIN_DIR=${bin}" "DIST_DIR=${dist}" "$@" bash "${script_dir}/build-dist.sh" \
    > "${work}/${name}.out" 2> "${work}/${name}.err"
  rc=$?
  set -e
  printf '%s\n' "${rc}" > "${work}/${name}.exit"
}

build_release build-dist
if [[ "$(cat "${work}/build-dist.exit")" -ne 0 ]]; then
  printf 'error: tools/scripts/build-dist.sh exited %s — see %s and %s\n' \
    "$(cat "${work}/build-dist.exit")" "${work}/build-dist.out" "${work}/build-dist.err" >&2
  exit 1
fi

# ---- dist layout -----------------------------------------------------------

function case_layout_stage() {
  local stage="${dist}/${ver_dir}"
  local expected="${work}/expected-stage"
  printf '%s\n' CONVENTIONS.md LICENSE README.md \
    kbase-darwin-arm64 kbase-darwin-arm64.asc \
    kbase-linux-amd64 kbase-linux-amd64.asc \
    kbase-windows-amd64.exe kbase-windows-amd64.exe.asc > "${expected}"
  diff "${expected}" <(ls -1 "${stage}")
}

function case_layout_dist() {
  local expected="${work}/expected-dist"
  printf '%s\n' "${installer}" "${installer_asc}" "${ver_dir}" "${tgz}" "${tgz_asc}" > "${expected}"
  diff "${expected}" <(ls -1 "${dist}")
}

function case_layout_tarball() {
  local entries="${work}/tarball-entries" outside count
  tar -tzf "${dist}/${tgz}" > "${entries}" || return 1
  count="$(wc -l < "${entries}" | tr -d ' ')"
  if [[ "${count}" -ne 10 ]]; then
    printf 'expected 10 entries, found %s:\n' "${count}"
    cat "${entries}"
    return 1
  fi
  outside="$(grep -v "^${ver_dir}/" "${entries}" || true)"
  if [[ -n "${outside}" ]]; then
    printf 'entries outside %s/:\n%s\n' "${ver_dir}" "${outside}"
    return 1
  fi
}

# case_layout_report [run]: checks the last line of the named run's output
# (default: the base build).
function case_layout_report() {
  local reported fields
  reported="$(tail -n 1 "${work}/${1:-build-dist}.out")"
  read -r -a fields <<< "${reported}"
  if [[ "${#fields[@]}" -ne 5 || "${fields[0]}" != "dist:" ]]; then
    printf 'report line: %s\n' "${reported}"
    return 1
  fi
  if [[ "${fields[1]}" != "${dist}/${tgz}" || "${fields[2]}" != "${dist}/${tgz_asc}" \
    || "${fields[3]}" != "${dist}/${installer}" || "${fields[4]}" != "${dist}/${installer_asc}" ]]; then
    printf 'report line: %s\nexpected:    %s\n' "${reported}" \
      "dist: ${dist}/${tgz} ${dist}/${tgz_asc} ${dist}/${installer} ${dist}/${installer_asc}"
    return 1
  fi
}

function case_build_invocations() {
  local log="${work}/stub/go.log" spec os arch name built line
  built="$(grep -c ' go build ' "${log}" || true)"
  if [[ "${built}" -ne 3 ]]; then
    printf 'expected 3 go build invocations, found %s:\n' "${built}"
    cat "${log}"
    return 1
  fi
  for spec in "darwin arm64 kbase-darwin-arm64" "linux amd64 kbase-linux-amd64" "windows amd64 kbase-windows-amd64.exe"; do
    read -r os arch name <<< "${spec}"
    line="GOOS=${os} GOARCH=${arch} go build -trimpath -tags kbasedist -ldflags=-s -w -o ${bin}/${name} ./cmd"
    if ! grep -qxF "${line}" "${log}"; then
      printf 'missing invocation: %s\n' "${line}"
      cat "${log}"
      return 1
    fi
  done
}

# ---- the generated installer ----------------------------------------------

function case_installer_syntax() {
  bash -n "${dist}/${installer}"
}

function case_installer_mode() {
  local mode
  mode="$(file_mode "${dist}/${installer}")"
  if [[ "${mode}" != "755" ]]; then
    printf 'mode: %s\n' "${mode}"
    return 1
  fi
}

# ---- case setup ------------------------------------------------------------

# prepare_case <dir> <keyring trust>: a fresh directory and a stub state whose
# keyring and signature packet name signer_fpr with that ownertrust, both logs
# empty.
function prepare_case() {
  local dir="${1}" trust="${2}"
  command rm -rf "${dir}"
  mkdir -p "${dir}/stub"
  printf '%s\t%s\t%s\n' "${signer_fpr}" "${signer_uid}" "${trust}" > "${dir}/stub/keyring.tsv"
  printf ':signature packet: algo 22, keyid %s\n\tversion 4\n\tissuer fpr v4 %s\n' \
    "${signer_fpr: -16}" "${signer_fpr}" > "${dir}/stub/packets"
  : > "${dir}/stub/gpg.log"
  : > "${dir}/stub/curl.log"
}

# prepare_install <dir> <keyring trust>: prepare_case plus the release files
# beside the installer.
function prepare_install() {
  prepare_case "${1}" "${2}"
  cp "${dist}/${tgz}" "${dist}/${tgz_asc}" "${dist}/${installer}" "${dist}/${installer_asc}" "${1}/"
}

# serve_file <dir> <path>...: stage files for the curl stub, which answers by
# basename and 404s every name it was not given.
function serve_file() {
  local dir="${1}" path
  shift
  mkdir -p "${dir}/stub/serve"
  for path in "$@"; do
    cp "${path}" "${dir}/stub/serve/" || return 1
  done
}

# run_installer <dir> <uname -s> <uname -m>: the installer resolves its release
# files from the working directory, so it runs in the case directory; stdin is
# /dev/null.
function run_installer() {
  local dir="${1}" rc
  set +e
  ( cd "${dir}" && env "PATH=${stubs}:${PATH}" "STUB_STATE_DIR=${dir}/stub" \
      "STUB_UNAME_S=${2}" "STUB_UNAME_M=${3}" bash "./${installer}" ) \
    > "${dir}/stdout" 2> "${dir}/stderr" < /dev/null
  rc=$?
  set -e
  printf '%s\n' "${rc}" > "${dir}/exit"
}

# run_installer_interactive <dir> <uname -s> <uname -m> <answer>: the prompt is
# answered over a pty, and everything the installer writes lands in
# <dir>/output.
function run_installer_interactive() {
  local dir="${1}" rc
  set +e
  env "PATH=${stubs}:${PATH}" "STUB_STATE_DIR=${dir}/stub" "STUB_UNAME_S=${2}" "STUB_UNAME_M=${3}" \
    "CASE_DIR=${dir}" "INSTALLER=./${installer}" "CASE_LOG=${dir}/pty.log" "ANSWER=${4}" \
    "${expect_bin}" -f "${pty_script}" > "${dir}/expect.out" 2> "${dir}/expect.err"
  rc=$?
  set -e
  printf '%s\n' "${rc}" > "${dir}/exit"
  if [[ -f "${dir}/pty.log" ]]; then
    tr -d '\r' < "${dir}/pty.log" > "${dir}/output"
  fi
}

function installer_rc() {
  cat "${1}/exit"
}

function verify_count() {
  grep -c '^gpg --verify' "${1}/stub/gpg.log" || true
}

function ownertrust_writes() {
  cat "${1}/stub/ownertrust.log" 2>/dev/null || true
}

function platform_install() {
  local dir="${1}" copied="${2}"
  prepare_install "${dir}" "u"
  run_installer "${dir}" "${3}" "${4}"
  if [[ "$(installer_rc "${dir}")" -ne 0 ]]; then
    printf 'exit %s\n' "$(installer_rc "${dir}")"
    cat "${dir}/stderr"
    return 1
  fi
  if [[ ! -f "${dir}/${ver_dir}/${copied}" ]]; then
    printf '%s not installed:\n' "${copied}"
    ls -1 "${dir}/${ver_dir}"
    return 1
  fi
}

function platform_refuse() {
  local dir="${1}"
  prepare_install "${dir}" "u"
  run_installer "${dir}" "${2}" "${3}"
  if [[ "$(installer_rc "${dir}")" -eq 0 ]]; then
    printf 'exit 0, expected a refusal\n'
    return 1
  fi
  if [[ "$(verify_count "${dir}")" -ne 0 ]]; then
    printf '%s verifications, expected none\n' "$(verify_count "${dir}")"
    return 1
  fi
  if [[ -s "${dir}/stub/curl.log" ]]; then
    printf 'the installer downloaded before refusing:\n'
    cat "${dir}/stub/curl.log"
    return 1
  fi
}

# ---- platforms -------------------------------------------------------------

function case_platform_darwin_arm64() {
  platform_install "${work}/platform-darwin-arm64" kbase Darwin arm64
}

function case_platform_linux_x86_64() {
  platform_install "${work}/platform-linux-x86_64" kbase Linux x86_64
}

function case_platform_mingw64_x86_64() {
  platform_install "${work}/platform-mingw64-x86_64" kbase.exe "MINGW64_NT-10.0-19045" x86_64
}

function case_platform_msys_amd64() {
  platform_install "${work}/platform-msys-amd64" kbase.exe "MSYS_NT-10.0-19045" amd64
}

function case_platform_refuse_darwin_x86_64() {
  platform_refuse "${work}/platform-refuse-darwin-x86_64" Darwin x86_64
}

function case_platform_refuse_linux_aarch64() {
  platform_refuse "${work}/platform-refuse-linux-aarch64" Linux aarch64
}

function case_platform_refuse_mingw64_arm64() {
  platform_refuse "${work}/platform-refuse-mingw64-arm64" "MINGW64_NT-10.0-19045" arm64
}

function case_platform_refuse_msys_arm64() {
  platform_refuse "${work}/platform-refuse-msys-arm64" "MSYS_NT-10.0-19045" arm64
}

function case_platform_refuse_unknown_os() {
  platform_refuse "${work}/platform-refuse-unknown-os" FreeBSD amd64
}

# ---- trust -----------------------------------------------------------------

function case_trust_ultimate() {
  local dir="${work}/trust-ultimate"
  prepare_install "${dir}" "u"
  run_installer "${dir}" Darwin arm64
  if [[ "$(installer_rc "${dir}")" -ne 0 ]]; then
    printf 'exit %s\n' "$(installer_rc "${dir}")"
    cat "${dir}/stderr"
    return 1
  fi
  if grep -qF 'Choice [y/o/N]:' "${dir}/stderr"; then
    printf 'prompted over a key that is already ultimate:\n'
    cat "${dir}/stderr"
    return 1
  fi
  if [[ -n "$(ownertrust_writes "${dir}")" ]]; then
    printf 'wrote ownertrust:\n%s\n' "$(ownertrust_writes "${dir}")"
    return 1
  fi
  if [[ "$(verify_count "${dir}")" -ne 5 ]]; then
    printf '%s verifications, expected 5\n' "$(verify_count "${dir}")"
    cat "${dir}/stub/gpg.log"
    return 1
  fi
}

function case_trust_answer_y() {
  local dir="${work}/trust-answer-y"
  prepare_install "${dir}" "-"
  run_installer_interactive "${dir}" Darwin arm64 y
  if [[ "$(installer_rc "${dir}")" -ne 0 ]]; then
    printf 'exit %s\n' "$(installer_rc "${dir}")"
    cat "${dir}/output"
    return 1
  fi
  if [[ "$(ownertrust_writes "${dir}")" != "${signer_fpr}:6:" ]]; then
    printf 'ownertrust: %s\nexpected:   %s\n' "$(ownertrust_writes "${dir}")" "${signer_fpr}:6:"
    return 1
  fi
  if [[ "$(verify_count "${dir}")" -ne 5 ]]; then
    printf '%s verifications, expected 5\n' "$(verify_count "${dir}")"
    cat "${dir}/stub/gpg.log"
    return 1
  fi
}

function case_trust_answer_o() {
  local dir="${work}/trust-answer-o"
  prepare_install "${dir}" "-"
  run_installer_interactive "${dir}" Darwin arm64 o
  if [[ "$(installer_rc "${dir}")" -ne 0 ]]; then
    printf 'exit %s\n' "$(installer_rc "${dir}")"
    cat "${dir}/output"
    return 1
  fi
  if [[ -n "$(ownertrust_writes "${dir}")" ]]; then
    printf 'wrote ownertrust:\n%s\n' "$(ownertrust_writes "${dir}")"
    return 1
  fi
  if [[ "$(verify_count "${dir}")" -ne 5 ]]; then
    printf '%s verifications, expected 5\n' "$(verify_count "${dir}")"
    cat "${dir}/stub/gpg.log"
    return 1
  fi
}

function case_trust_answer_n() {
  local dir="${work}/trust-answer-n"
  prepare_install "${dir}" "-"
  run_installer_interactive "${dir}" Darwin arm64 n
  if [[ "$(installer_rc "${dir}")" -eq 0 ]]; then
    printf 'exit 0, expected a refusal\n'
    cat "${dir}/output"
    return 1
  fi
  if [[ -n "$(ownertrust_writes "${dir}")" ]]; then
    printf 'wrote ownertrust:\n%s\n' "$(ownertrust_writes "${dir}")"
    return 1
  fi
  if [[ "$(verify_count "${dir}")" -ne 0 ]]; then
    printf '%s verifications, expected none\n' "$(verify_count "${dir}")"
    return 1
  fi
}

function case_trust_signer_absent() {
  local dir="${work}/trust-signer-absent"
  prepare_install "${dir}" "u"
  printf '%s\t%s\t%s\n' "${other_fpr}" "${other_uid}" "u" > "${dir}/stub/keyring.tsv"
  run_installer "${dir}" Darwin arm64
  if [[ "$(installer_rc "${dir}")" -eq 0 ]]; then
    printf 'exit 0, expected a refusal\n'
    return 1
  fi
  if [[ "$(verify_count "${dir}")" -ne 0 ]]; then
    printf '%s verifications, expected none\n' "$(verify_count "${dir}")"
    return 1
  fi
  if [[ -e "${dir}/${ver_dir}" ]]; then
    printf 'the tarball was extracted before the key was known to be in the keyring:\n'
    ls -1 "${dir}/${ver_dir}"
    return 1
  fi
  if [[ -s "${dir}/stub/curl.log" ]]; then
    printf 'the installer downloaded before refusing:\n'
    cat "${dir}/stub/curl.log"
    return 1
  fi
}

function case_trust_signer_untrusted() {
  local dir="${work}/trust-signer-untrusted"
  prepare_install "${dir}" "-"
  printf '%s\t%s\t%s\n%s\t%s\t%s\n' \
    "${other_fpr}" "${other_uid}" "u" \
    "${signer_fpr}" "${signer_uid}" "-" > "${dir}/stub/keyring.tsv"
  run_installer_interactive "${dir}" Darwin arm64 y
  if [[ "$(installer_rc "${dir}")" -ne 0 ]]; then
    printf 'exit %s\n' "$(installer_rc "${dir}")"
    cat "${dir}/output"
    return 1
  fi
  if [[ "$(ownertrust_writes "${dir}")" != "${signer_fpr}:6:" ]]; then
    printf 'ownertrust: %s\nexpected:   %s\n' "$(ownertrust_writes "${dir}")" "${signer_fpr}:6:"
    return 1
  fi
  if [[ "$(verify_count "${dir}")" -ne 5 ]]; then
    printf '%s verifications, expected 5\n' "$(verify_count "${dir}")"
    cat "${dir}/stub/gpg.log"
    return 1
  fi
}

function case_trust_stdin_not_terminal() {
  local dir="${work}/trust-stdin-not-terminal"
  prepare_install "${dir}" "-"
  run_installer "${dir}" Darwin arm64
  if [[ "$(installer_rc "${dir}")" -eq 0 ]]; then
    printf 'exit 0, expected a refusal\n'
    return 1
  fi
  if grep -qF 'Choice' "${dir}/stderr"; then
    printf 'prompted with stdin not a terminal:\n'
    cat "${dir}/stderr"
    return 1
  fi
  if [[ "$(verify_count "${dir}")" -ne 0 ]]; then
    printf '%s verifications, expected none\n' "$(verify_count "${dir}")"
    return 1
  fi
}

# ---- signatures ------------------------------------------------------------

# refused_before_extract <dir>: the installer exited non-zero and left nothing
# extracted.
function refused_before_extract() {
  local dir="${1}"
  if [[ "$(installer_rc "${dir}")" -eq 0 ]]; then
    printf 'exit 0, expected a refusal\n'
    cat "${dir}/stderr"
    return 1
  fi
  if [[ -e "${dir}/${ver_dir}" ]]; then
    printf 'the release was extracted:\n'
    ls -1 "${dir}/${ver_dir}"
    return 1
  fi
}

function case_signature_tampered_tarball() {
  local dir="${work}/signature-tampered-tarball"
  prepare_install "${dir}" "u"
  printf 'tampered\n' >> "${dir}/${tgz}" || return 1
  run_installer "${dir}" Darwin arm64
  refused_before_extract "${dir}"
}

# retar_with_swapped_asc <dir>: writes <dir>'s tarball with
# kbase-linux-amd64's signature replaced by kbase-darwin-arm64's, then signs
# the tarball with the stub, so only the binary's signature is wrong.
function retar_with_swapped_asc() {
  local dir="${1}" stage="${work}/retar-swapped-asc"
  command rm -rf "${stage}"
  mkdir -p "${stage}"
  tar -xzf "${dist}/${tgz}" -C "${stage}" || return 1
  cp "${stage}/${ver_dir}/kbase-darwin-arm64.asc" "${stage}/${ver_dir}/kbase-linux-amd64.asc" || return 1
  tar -czf "${dir}/${tgz}" -C "${stage}" "${ver_dir}" || return 1
  env "PATH=${stubs}:${PATH}" "STUB_STATE_DIR=${dir}/stub" \
    gpg --yes --local-user "${signer_fpr}" --detach-sign --armor "${dir}/${tgz}" || return 1
}

function case_signature_asc_for_another_file() {
  local dir="${work}/signature-asc-for-another-file"
  prepare_install "${dir}" "u"
  retar_with_swapped_asc "${dir}" || return 1
  run_installer "${dir}" Darwin arm64
  if [[ "$(installer_rc "${dir}")" -eq 0 ]]; then
    printf 'exit 0, expected a refusal\n'
    cat "${dir}/stderr"
    return 1
  fi
  if [[ -e "${dir}/${ver_dir}/kbase" ]]; then
    printf 'a binary was copied from a signature that names another file\n'
    return 1
  fi
  if [[ "$(tail -n 1 "${dir}/stub/gpg.log")" != "gpg --verify ./${ver_dir}/kbase-linux-amd64.asc ./${ver_dir}/kbase-linux-amd64" ]]; then
    printf 'the installer did not stop at the mismatched signature:\n'
    cat "${dir}/stub/gpg.log"
    return 1
  fi
}

# ---- downloads -------------------------------------------------------------

function case_download_succeeds() {
  local dir="${work}/download-succeeds"
  prepare_case "${dir}" "u"
  cp "${dist}/${installer}" "${dir}/"
  serve_file "${dir}" "${dist}/${installer_asc}" "${dist}/${tgz}" "${dist}/${tgz_asc}"
  run_installer "${dir}" Darwin arm64
  if [[ "$(installer_rc "${dir}")" -ne 0 ]]; then
    printf 'exit %s\n' "$(installer_rc "${dir}")"
    cat "${dir}/stderr"
    return 1
  fi
  if [[ ! -f "${dir}/${ver_dir}/kbase" ]]; then
    printf 'nothing was installed:\n'
    ls -1 "${dir}"
    return 1
  fi
  if [[ "$(wc -l < "${dir}/stub/curl.log" | tr -d ' ')" -ne 3 ]]; then
    printf 'expected 3 downloads, the script, the tarball and its signature:\n'
    cat "${dir}/stub/curl.log"
    return 1
  fi
  if grep -vE -- '^curl --fail .* --proto =https --max-time [0-9]+ -O https://' "${dir}/stub/curl.log"; then
    printf 'the download above is not --fail --proto =https --max-time bounded\n'
    return 1
  fi
}

function case_download_404_not_a_signature() {
  local dir="${work}/download-404-body"
  prepare_case "${dir}" "u"
  cp "${dist}/${installer}" "${dir}/"
  serve_file "${dir}" "${dist}/${tgz}" "${dist}/${tgz_asc}"
  run_installer "${dir}" Darwin arm64
  refused_before_extract "${dir}" || return 1
  if [[ -e "${dir}/${installer_asc}" ]]; then
    printf 'the 404 body was written where the signature belongs:\n'
    cat "${dir}/${installer_asc}"
    return 1
  fi
}

# ---- dist reruns and the version string ------------------------------------

function case_dist_rerunnable() {
  build_release dist-rerun
  if [[ "$(cat "${work}/dist-rerun.exit")" -ne 0 ]]; then
    printf 'a second run at %s exited %s\n' "${version}" "$(cat "${work}/dist-rerun.exit")"
    cat "${work}/dist-rerun.err"
    return 1
  fi
  case_layout_dist
}

function case_version_refused() {
  local wanted
  for wanted in "2.0.0-rc1" '2.0.0$(touch '"${work}"'/version-escaped)'; do
    build_release version-refused "${wanted}"
    if [[ "$(cat "${work}/version-refused.exit")" -eq 0 ]]; then
      printf 'version %s was accepted\n' "${wanted}"
      return 1
    fi
    if [[ -e "${work}/version-escaped" ]]; then
      printf 'version %s reached the shell\n' "${wanted}"
      return 1
    fi
    if [[ ! -f "${dist}/${tgz}" ]]; then
      printf 'version %s was refused only after the previous release was removed\n' "${wanted}"
      return 1
    fi
  done
}

# ---- tagging ---------------------------------------------------------------

# tag_run <name> [NAME=value...]: a build with its own stub state, so its logs
# hold this run's calls only.
function tag_run() {
  local name="${1}"
  shift
  mkdir -p "${work}/stub-${name}"
  build_release "${name}" "${go_version}" "STUB_STATE_DIR=${work}/stub-${name}" "$@"
}

function case_tag_after_last_stage() {
  local state="${work}/stub-tag" signs
  tag_run tag TAG=1
  if [[ "$(cat "${work}/tag.exit")" -ne 0 ]]; then
    cat "${work}/tag.err"
    return 1
  fi
  signs="$(wc -l < "${state}/gpg.log" | tr -d ' ')"
  if [[ "${signs}" -ne 5 ]]; then
    printf 'expected 5 signing calls, found %s\n' "${signs}"
    return 1
  fi
  if [[ "$(grep -c '^git tag' "${state}/git.log")" -ne 1 ]] \
    || ! grep -qxF "git tag ${version}" "${state}/git.log" \
    || [[ "$(tail -n 1 "${state}/git.log")" != "tagged ${version} after ${signs} gpg calls" ]]; then
    printf 'the tag was not the single, last call after all %s signatures:\n' "${signs}"
    cat "${state}/git.log"
    return 1
  fi
  case_layout_report tag
}

# tag_refused <name> NAME=value...: the run fails with the usage refusal, builds
# nothing and tags nothing.
function tag_refused() {
  local name="${1}" state="${work}/stub-${1}"
  tag_run "$@"
  if [[ "$(cat "${work}/${name}.exit")" -eq 0 ]]; then
    printf 'the run was accepted\n'
    return 1
  fi
  if grep -q ' go build ' "${state}/go.log" || grep -q '^git tag' "${state}/git.log"; then
    printf 'the refused run built or tagged:\n'
    cat "${state}/go.log" "${state}/git.log"
    return 1
  fi
}

function case_tag_refused_when_tag_exists() {
  tag_refused tag-exists TAG=1 STUB_GIT_TAG_EXISTS=1
}

function case_tag_refused_when_tree_dirty() {
  tag_refused tag-dirty TAG=1 "STUB_GIT_DIRTY= M cmd/main.go"
}

function case_no_tag_without_the_option() {
  local state="${work}/stub-no-tag"
  tag_run no-tag
  if [[ "$(cat "${work}/no-tag.exit")" -ne 0 ]]; then
    cat "${work}/no-tag.err"
    return 1
  fi
  if [[ -e "${state}/git.log" ]]; then
    printf 'git was called without TAG:\n'
    cat "${state}/git.log"
    return 1
  fi
}

# ---- cases -----------------------------------------------------------------

run_case layout-stage case_layout_stage
run_case layout-dist case_layout_dist
run_case layout-tarball case_layout_tarball
run_case layout-report case_layout_report
run_case build-invocations case_build_invocations
run_case installer-syntax case_installer_syntax
run_case installer-mode case_installer_mode
run_case version-refused case_version_refused
run_case dist-rerunnable case_dist_rerunnable
run_case tag-after-last-stage case_tag_after_last_stage
run_case tag-refused-when-tag-exists case_tag_refused_when_tag_exists
run_case tag-refused-when-tree-dirty case_tag_refused_when_tree_dirty
run_case no-tag-without-the-option case_no_tag_without_the_option
run_case platform-darwin-arm64 case_platform_darwin_arm64
run_case platform-linux-x86_64 case_platform_linux_x86_64
run_case platform-mingw64-x86_64 case_platform_mingw64_x86_64
run_case platform-msys-amd64 case_platform_msys_amd64
run_case platform-refuse-darwin-x86_64 case_platform_refuse_darwin_x86_64
run_case platform-refuse-linux-aarch64 case_platform_refuse_linux_aarch64
run_case platform-refuse-mingw64-arm64 case_platform_refuse_mingw64_arm64
run_case platform-refuse-msys-arm64 case_platform_refuse_msys_arm64
run_case platform-refuse-unknown-os case_platform_refuse_unknown_os
run_case trust-ultimate case_trust_ultimate
run_case trust-answer-y case_trust_answer_y
run_case trust-answer-o case_trust_answer_o
run_case trust-answer-n case_trust_answer_n
run_case trust-signer-absent case_trust_signer_absent
run_case trust-signer-untrusted case_trust_signer_untrusted
run_case trust-stdin-not-terminal case_trust_stdin_not_terminal
run_case signature-tampered-tarball case_signature_tampered_tarball
run_case signature-asc-for-another-file case_signature_asc_for_another_file
run_case download-succeeds case_download_succeeds
run_case download-404-not-a-signature case_download_404_not_a_signature

printf '\n%s passed, %s failed\n' "${passed}" "${failed}"
if [[ "${failed}" -ne 0 ]]; then
  exit 1
fi
