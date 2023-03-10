#!/bin/bash

# Run Oasis CLI as defined in $1 and store its output to $2.
# Appends "-y -o /dev/null" to command execution, if filename $1 contains .y.

set -euo pipefail

OASIS_CMD=$(readlink -f ./oasis)
IN=$(readlink -f $1)
touch "$2" # MacOS: readlink -f does not work if file does not exist. readlink -m is not implemented yet.
OUT=$(readlink -f $2)
EXAMPLE_NAME="$(basename $(dirname $IN))"
CUSTOM_CFG_DIR="$(dirname $IN)/config"
CFG_DIR="/tmp/cli_examples/${EXAMPLE_NAME}"
CFG_FLAG="--config /tmp/cli_examples/${EXAMPLE_NAME}/cli.toml"
USER_CFG_DIR="${HOME}/.config/oasis"
RESTORE_CFG_DIR="${HOME}/.config/oasis.backup"

# TODO: Is there a standardized way to determine $XDG_CONFIG_HOME?
if [ $(uname -s) == "Darwin" ]
then
  USER_CFG_DIR="${HOME}/Library/Application Support/oasis"
  RESTORE_CFG_DIR="${HOME}/Library/Application Support/oasis.backup"
fi

# Check, if the input filename ends with .y.in and append -y -o /dev/null.
# This is useful e.g. for signing the transactions by both not broadcasting
# anything so we don't break the state and that we still have a clean input file
# that can be included in the documentation.
YES_FLAG=""
if [ "$(echo "$IN" | cut -d "." -f 2)" == "y" ]; then
  YES_FLAG="-y -o /dev/null"
fi

echo "Running ${EXAMPLE_NAME}/$(basename $IN):"

# Prepare clean config file for the example or take the example-specific one, if
# it exists.
function init_cfg() {
  # Init config in the first scenario step only.
  if [ "$IN" != "$(ls $(dirname $IN)/*.in | head -n1)" ]; then
    return
  fi

  rm -rf "${CFG_DIR}"
  mkdir -p "$(dirname ${CFG_DIR})"

  # Check for example-specific config and copy it over.
  if [ -d "${CUSTOM_CFG_DIR}" ]; then
    cp -r "${CUSTOM_CFG_DIR}" "${CFG_DIR}"
    return
  fi

  # Otherwise, generate a clean config file.
  if [ -d "${USER_CFG_DIR}" ]; then
    if [ -d "${RESTORE_CFG_DIR}" ]; then
      echo "error: cannot initialize config: restore config directory ${RESTORE_CFG_DIR} already exists. Please restore it into your ${USER_CFG_DIR} or remove it"
      exit 1
    fi
    mv "${USER_CFG_DIR}" "${RESTORE_CFG_DIR}"
  fi

  # XXX: What is the simplest Oasis CLI command to generate initial config file?
  ${OASIS_CMD} network ls >/dev/null
  wait

  # Use the fresh config for our example.
  mv "${USER_CFG_DIR}" "${CFG_DIR}"

  # Restore the original config, if it existed.
  if [ -d "${RESTORE_CFG_DIR}" ]; then
    mv "${RESTORE_CFG_DIR}" "${USER_CFG_DIR}"
  fi
}

init_cfg

# Replace "oasis" with the actual path to the Oasis CLI executable.
CMD="$(sed "s#oasis#$OASIS_CMD#" $IN) ${CFG_FLAG} ${YES_FLAG}"
echo "  ${CMD}"

# Use UTC timezone.
export TZ=UTC

# Execute the Oasis CLI and store the PID.
cd $(dirname ${IN})
${CMD} > ${OUT}

# Trim last two lines, if in non-interactive mode ("Sign this transaction?" and
# "In case you are using a hardware-based signer...").
if [ "$YES_FLAG" != "" ] && grep -q "Sign this transaction?" "${OUT}"
then
  OUT_LINES=$(( $(cat ${OUT} | wc -l)-2 )) # MacOS: head -n -2 is not implemented yet
  head -n ${OUT_LINES} ${OUT} > ${OUT}.tmp
  mv ${OUT}.tmp ${OUT}
fi
