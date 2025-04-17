#! /bin/sh

# based on https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3

: ${BINARY_NAME:="sichek"}
: ${USE_SUDO:="true"}
: ${DEBUG:="false"}
: ${VERIFY_CHECKSUM:="false"}
: ${SICHEK_INSTALL_DIR:="/usr/sbin"}

HAS_CURL="$(type "curl" &> /dev/null && echo true || echo false)"
HAS_WGET="$(type "wget" &> /dev/null && echo true || echo false)"

# initArch discovers the architecture for this system.
initArch() {
  ARCH=$(uname -m)
  case $ARCH in
    armv5*) ARCH="armv5";;
    armv6*) ARCH="armv6";;
    armv7*) ARCH="arm";;
    aarch64) ARCH="arm64";;
    x86) ARCH="386";;
    x86_64) ARCH="amd64";;
    i686) ARCH="386";;
    i386) ARCH="386";;
  esac
}

# initOS discovers the operating system for this system.
initOS() {
  OS=$(echo `uname`|tr '[:upper:]' '[:lower:]')

  case "$OS" in
    # Minimalist GNU for Windows
    mingw*|cygwin*) OS='windows';;
  esac
  if [ "$OS" = "linux" ]; then
    if [ -f /etc/os-release ]; then
      . /etc/os-release
      case "$ID" in
        ubuntu) OSRELEASE='ubuntu';;
        centos) OSRELEASE='centos';;
        *) OSRELEASE='';;
      esac
    fi
  fi
}

initLibc() {
  # Attempt to get the libc version
  if ldd --version 2>/dev/null | grep -q 'GNU'; then
    # GNU libc version
    libc_version=$(ldd --version | head -n1 | awk '{print $NF}')
  elif [ -f /lib/x86_64-linux-gnu/libc.so.6 ]; then
    # Check directly from libc binary
    libc_version=$(/lib/x86_64-linux-gnu/libc.so.6 | head -n1 | awk '{print $NF}')
  elif [ -f /lib/libc.so.6 ]; then
    # Fallback for 32-bit systems or other setups
    libc_version=$(/lib/libc.so.6 | head -n1 | awk '{print $NF}')
  else
    echo "libc version could not be determined."
    return 1
  fi
  # Compare the version using version sorting
  required_version="2.35"
  if [[ $(echo -e "$libc_version\n$required_version" | sort -V | head -n1) != "$required_version" ]]; then
    echo "libc version needs to be $required_version or above"
    return 1
  fi
  echo "Detected libc version: $libc_version"
}

# runs the given command as root (detects if we are root already)
runAsRoot() {
  if [ $EUID -ne 0 -a "$USE_SUDO" = "true" ]; then
    sudo "${@}"
  else
    "${@}"
  fi
}

# verifySupported checks that the os/arch combination is supported for
# binary builds, as well whether or not necessary tools are present.
verifySupported() {
#   local supported="darwin-amd64\ndarwin-arm64\nlinux-386\nlinux-amd64\nlinux-arm\nlinux-arm64\nlinux-ppc64le\nlinux-s390x\nlinux-riscv64\nwindows-amd64\nwindows-arm64"
  local supported="linux-amd64"
  if ! echo "${supported}" | grep -q "${OS}-${ARCH}"; then
    echo "No prebuilt binary for ${OS}-${ARCH}."
    echo "To build from source, go to https://github.com/scitix/sichek"
    exit 1
  fi

  if [ "$OSRELEASE" != "ubuntu" ] && [ "$OSRELEASE" != "centos" ]; then
    echo "No prebuilt binary for ${OSRELEASE}."
    echo "To build from source, go to https://github.com/scitix/sichek"
  fi

  if [ "${HAS_CURL}" != "true" ] && [ "${HAS_WGET}" != "true" ]; then
    echo "Either curl or wget is required"
    exit 1
  fi

#   if [ "${VERIFY_CHECKSUM}" == "true" ] && [ "${HAS_OPENSSL}" != "true" ]; then
#     echo "In order to verify checksum, openssl must first be installed."
#     echo "Please install openssl or set VERIFY_CHECKSUM=false in your environment."
#     exit 1
#   fi

#   if [ "${VERIFY_SIGNATURES}" == "true" ]; then
#     if [ "${HAS_GPG}" != "true" ]; then
#       echo "In order to verify signatures, gpg must first be installed."
#       echo "Please install gpg or set VERIFY_SIGNATURES=false in your environment."
#       exit 1
#     fi
#     if [ "${OS}" != "linux" ]; then
#       echo "Signature verification is currently only supported on Linux."
#       echo "Please set VERIFY_SIGNATURES=false or verify the signatures manually."
#       exit 1
#     fi
#   fi

#   if [ "${HAS_GIT}" != "true" ]; then
#     echo "[WARNING] Could not find git. It is required for plugin installation."
#   fi

#   if [ "${HAS_TAR}" != "true" ]; then
#     echo "[ERROR] Could not find tar. It is required to extract the helm binary archive."
#     exit 1
#   fi
}

# checkDesiredVersion checks if the desired version is available.
checkDesiredVersion() {
  if [ "x$DESIRED_VERSION" == "x" ]; then
    DESIRED_VERSION="latest"
    # # Get tag from release URL
    # local latest_release_url="https://oss-ap-southeast.scitix.ai/scitix-release/sichek/latest/"
    # local latest_release_response=""
    # if [ "${HAS_CURL}" == "true" ]; then
    #   latest_release_response=$( curl -L --silent --show-error --fail "$latest_release_url" 2>&1 || true )
    # elif [ "${HAS_WGET}" == "true" ]; then
    #   latest_release_response=$( wget "$latest_release_url" -q -O - 2>&1 || true )
    # fi
    # TAG=$( echo "$latest_release_response" | grep '^v[0-9]' )
    # if [ "x$TAG" == "x" ]; then
    #   printf "Could not retrieve the latest release tag information from %s: %s\n" "${latest_release_url}" "${latest_release_response}"
    #   exit 1
    # fi
  else
    TAG=$DESIRED_VERSION
  fi
}

# checkSichekInstalledVersion checks which version of helm is installed and
# # if it needs to be changed.
# checkSichekInstalledVersion() {
#   if [[ -f "${SICHEK_INSTALL_DIR}/${BINARY_NAME}" ]]; then
#     local version=$("${SICHEK_INSTALL_DIR}/${BINARY_NAME}" version --template="{{ .Version }}")
#     if [[ "$version" == "$TAG" ]]; then
#       echo "Helm ${version} is already ${DESIRED_VERSION:-latest}"
#       return 0
#     else
#       echo "Helm ${TAG} is available. Changing from version ${version}."
#       return 1
#     fi
#   else
#     return 1
#   fi
# }


# downloadFile downloads the latest binary package and also the checksum
# for that binary.
downloadFile() {
  if [ $OSRELEASE = "ubuntu" ]; then
    SICHEK_DIST="sichek_${DESIRED_VERSION}_linux_amd64.deb"
  elif [ $OSRELEASE = "centos" ]; then
    SICHEK_DIST="sichek_${DESIRED_VERSION}_linux_amd64.rpm"
  fi
  DOWNLOAD_URL="https://oss-ap-southeast.scitix.ai/scitix-release/sichek/${DESIRED_VERSION}/${SICHEK_DIST}"
#   CHECKSUM_URL="$DOWNLOAD_URL.sha256"
  SICHEK_TMP_ROOT="$(mktemp -dt sichek-installer-XXXXXX)"
  SICHEK_TMP_FILE="$SICHEK_TMP_ROOT/$SICHEK_DIST"
#   SICHEK_SUM_FILE="$SICHEK_TMP_ROOT/$SICHEK_DIST.sha256"
  echo "Downloading $DOWNLOAD_URL"
  set +e
  if [ "${HAS_CURL}" == "true" ]; then
    # curl -SsL "$CHECKSUM_URL" -o "$SICHEK_SUM_FILE"
    curl -SsL "$DOWNLOAD_URL" -o "$SICHEK_TMP_FILE"
  elif [ "${HAS_WGET}" == "true" ]; then
    # wget -q -O "$SICHEK_SUM_FILE" "$CHECKSUM_URL"
    wget -q -O "$SICHEK_TMP_FILE" "$DOWNLOAD_URL"
  fi
  set -e
}

# installFile installs the Helm binary.
installFile() {
  if [ $OSRELEASE = "ubuntu" ]; then
    runAsRoot dpkg -i "$SICHEK_TMP_FILE"
  elif [ $OSRELEASE = "centos" ]; then
    runAsRoot rpm -i "$SICHEK_TMP_FILE"
  fi
  echo "$BINARY_NAME has installed successfully"
}

# verifyChecksum verifies the SHA256 checksum of the binary package.
verifyChecksum() {
  if [ "${VERIFY_CHECKSUM}" == "true" ]; then
    printf "Verifying checksum... "
    local sum=$(openssl sha1 -sha256 ${HELM_TMP_FILE} | awk '{print $2}')
    local expected_sum=$(cat ${HELM_SUM_FILE})
    if [ "$sum" != "$expected_sum" ]; then
        echo "SHA sum of ${HELM_TMP_FILE} does not match. Aborting."
        exit 1
    fi
    echo "Done."
  fi
}

# fail_trap is executed if an error occurs.
fail_trap() {
  result=$?
  if [ "$result" != "0" ]; then
    if [[ -n "$INPUT_ARGUMENTS" ]]; then
      echo "Failed to install $BINARY_NAME with the arguments provided: $INPUT_ARGUMENTS"
      help
    else
      echo "Failed to install $BINARY_NAME"
    fi
    echo -e "\tFor support, go to https://github.com/scitix/sichek."
  fi
  cleanup
  exit $result
}

# testVersion tests the installed client to make sure it is working.
testVersion() {
  set +e
  ${BINARY_NAME} version
#   SICHEK="$(command -v $BINARY_NAME)"
  if [[ $? -ne 0 ]]; then
    echo "$BINARY_NAME not found. Is $SICHEK_INSTALL_DIR on your "'$PATH?'
    exit 1
  fi
  set -e
}

# help provides possible cli installation arguments
help () {
  echo "Accepted cli arguments are:"
  echo -e "\t[--help|-h ] ->> prints this help"
  echo -e "\t[--version|-v <desired_version>] . When not defined it fetches the latest release from GitHub"
  echo -e "\te.g. --version v0.1.0"
  echo -e "\t[--no-sudo]  ->> install without sudo"
}


# cleanup temporary files to avoid https://github.com/helm/helm/issues/2977
cleanup() {
  if [[ -d "${SICHEK_TMP_ROOT:-}" ]]; then
    rm -rf "$SICHEK_TMP_ROOT"
  fi
}

# Execution
#Stop execution on any error
trap "fail_trap" EXIT

set -e
# Set debug if desired
if [ "${DEBUG}" == "true" ]; then
  set -x
fi

# Parsing input arguments (if any)
export INPUT_ARGUMENTS="${@}"
set -u
while [[ $# -gt 0 ]]; do
  case $1 in
    '--version'|-v)
       shift
       if [[ $# -ne 0 ]]; then
           export DESIRED_VERSION="${1}"
           if [[ "$1" != "v"* ]]; then
               echo "Expected version arg ('${DESIRED_VERSION}') to begin with 'v', fixing..."
               export DESIRED_VERSION="v${1}"
           fi
       else
           echo -e "Please provide the desired version. e.g. --version v3.0.0 or -v canary"
           exit 0
       fi
       ;;
    '--no-sudo')
       USE_SUDO="false"
       ;;
    '--help'|-h)
       help
       exit 0
       ;;
    *) exit 1
       ;;
  esac
  shift
done
set +u


initArch
initOS
initLibc
verifySupported
checkDesiredVersion
downloadFile
verifyChecksum
installFile
testVersion
cleanup
