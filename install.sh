#!/bin/sh
# install.sh — install the work CLI binary
#
# Downloads the prebuilt archive for your OS/arch from a GitHub release,
# verifies its SHA-256 checksum, and installs `work` to --prefix.
#
# Pass --from-source to build from a local checkout instead (requires Go
# and a .git directory).

set -eu

DIR=$(cd "$(dirname "$0")" && pwd)
DRY_RUN=0
FROM_SOURCE=0
PREFIX="$HOME/.local/bin"
RELEASE_OWNER="gh-xj"
RELEASE_REPO="work-cli"
VERSION_REQUESTED=""

if [ -t 2 ] && [ -z "${NO_COLOR:-}" ]; then
    _C_INF=$(printf '\033[92mINF\033[0m')
    _C_WRN=$(printf '\033[93mWRN\033[0m')
    _C_ERR=$(printf '\033[91mERR\033[0m')
    _C_DIM=$(printf '\033[2m')
    _C_OFF=$(printf '\033[0m')
else
    _C_INF=INF; _C_WRN=WRN; _C_ERR=ERR; _C_DIM=""; _C_OFF=""
fi

log()  { printf '%s %s\n' "$_C_INF" "$*" >&2; }
warn() { printf '%s %s\n' "$_C_WRN" "$*" >&2; }
die()  { printf '%s %s\n' "$_C_ERR" "$*" >&2; exit 1; }

run() {
    if [ "$DRY_RUN" -eq 1 ]; then
        printf '%s %sdry-run%s %s\n' "$_C_INF" "$_C_DIM" "$_C_OFF" "$*" >&2
    else
        eval "$@"
    fi
}

usage() {
    cat <<'EOF'
Usage: ./install.sh [options]

Options:
  --prefix <dir>      binary install directory (default: ~/.local/bin)
  --version <vX.Y.Z>  install a specific tagged version (default: latest)
  --from-source       build from local checkout instead of downloading
  --dry-run           preview actions without making changes
  -h, --help          show this message and exit

Default behavior:
  - If --from-source, OR (Go on PATH AND .git present in script dir):
      build from source.
  - Else: download latest release archive and verify checksum.
EOF
}

while [ $# -gt 0 ]; do
    case "$1" in
        --prefix=*)   PREFIX="${1#*=}";   [ -n "$PREFIX" ]   || die "empty --prefix";   shift ;;
        --prefix)     [ $# -ge 2 ] && [ -n "$2" ] || die "missing value for --prefix";   PREFIX="$2";   shift 2 ;;
        --version=*)  VERSION_REQUESTED="${1#*=}"; shift ;;
        --version)    [ $# -ge 2 ] && [ -n "$2" ] || die "missing value for --version"; VERSION_REQUESTED="$2"; shift 2 ;;
        --from-source) FROM_SOURCE=1;     shift ;;
        --dry-run)     DRY_RUN=1;         shift ;;
        -h|--help)     usage; exit 0 ;;
        *) die "unknown arg: $1" ;;
    esac
done

detect_os() {
    case "$(uname -s)" in
        Darwin) OS=darwin ;;
        Linux)  OS=linux ;;
        *) die "unsupported OS: $(uname -s) (expected Darwin or Linux)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  ARCH=amd64 ;;
        arm64|aarch64) ARCH=arm64 ;;
        *) die "unsupported arch: $(uname -m)" ;;
    esac
}

resolve_strategy() {
    if [ "$FROM_SOURCE" -eq 1 ]; then
        STRATEGY=source
    elif command -v go >/dev/null 2>&1 && [ -d "$DIR/.git" ]; then
        STRATEGY=source
    else
        STRATEGY=download
    fi
}

_sha256() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$@"
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$@"
    else
        warn "no sha256 tool found; skipping checksum verification"
        return 0
    fi
}

verify_checksum() {
    checksums="$1"; archive_name="$2"; workdir="$3"
    if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
        warn "no sha256 tool found; skipping checksum verification"
        return 0
    fi
    grep "  ${archive_name}\$" "$checksums" >"$workdir/expected.txt" \
        || die "no checksum entry for $archive_name in checksums.txt"
    ( cd "$workdir" && _sha256 --check --status expected.txt ) \
        || die "checksum mismatch for $archive_name"
    log "checksum verified: $archive_name"
}

install_binary() {
    if [ "$STRATEGY" = source ]; then
        command -v go >/dev/null 2>&1 \
            || die "go toolchain not found; remove --from-source or install Go"
        log "building work" "prefix=$PREFIX"
        run "(cd \"$DIR\" && mkdir -p \"$PREFIX\" && go build -o \"$PREFIX/work\" ./cmd/work)"
        return 0
    fi

    if [ -n "$VERSION_REQUESTED" ]; then
        VERSION="${VERSION_REQUESTED#v}"
        log "requested version: v$VERSION"
    else
        api_url="https://api.github.com/repos/${RELEASE_OWNER}/${RELEASE_REPO}/releases/latest"
        if [ "$DRY_RUN" -eq 1 ]; then
            VERSION="<latest>"
            log "DRY-RUN: curl -sfL $api_url | grep tag_name | sed ... -> VERSION"
        else
            VERSION=$(curl -sfL "$api_url" 2>/dev/null \
                | grep '"tag_name"' \
                | sed -e 's/.*"tag_name"[[:space:]]*:[[:space:]]*"v\{0,1\}//' -e 's/".*//' \
                | head -n1) || VERSION=""
            [ -n "$VERSION" ] || die "could not determine latest release version from $api_url"
            log "latest release: v$VERSION"
        fi
    fi

    ARCHIVE="work-${VERSION}-${OS}-${ARCH}.tar.gz"
    base_url="https://github.com/${RELEASE_OWNER}/${RELEASE_REPO}/releases/download/v${VERSION}"
    TMPDIR=$(mktemp -d 2>/dev/null || mktemp -d -t work-install)
    trap 'rm -rf "$TMPDIR"' EXIT INT TERM

    run "curl -sfL \"${base_url}/${ARCHIVE}\" -o \"$TMPDIR/$ARCHIVE\""
    run "curl -sfL \"${base_url}/checksums.txt\" -o \"$TMPDIR/checksums.txt\""
    if [ "$DRY_RUN" -eq 1 ]; then
        log "DRY-RUN: verify_checksum $TMPDIR/checksums.txt $ARCHIVE"
    else
        verify_checksum "$TMPDIR/checksums.txt" "$ARCHIVE" "$TMPDIR"
    fi
    run "tar -xzf \"$TMPDIR/$ARCHIVE\" -C \"$TMPDIR\""
    run "mkdir -p \"$PREFIX\""
    run "mv \"$TMPDIR/work\" \"$PREFIX/work.new\""
    run "mv \"$PREFIX/work.new\" \"$PREFIX/work\""
    run "chmod +x \"$PREFIX/work\""
}

main() {
    detect_os
    detect_arch
    resolve_strategy
    log "starting install" "os=$OS arch=$ARCH strategy=$STRATEGY prefix=$PREFIX"
    install_binary
    cat <<EOF

Next steps:

  1. Ensure $PREFIX is on PATH.
  2. Try: work init && work inbox add "first capture"
EOF
    log "install complete" "prefix=$PREFIX"
}

main
