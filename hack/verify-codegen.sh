#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

PROJECT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
TIMESTAMP=`date +"%Y%m%d%H%M%S"`
TMPROOT="/tmp/coxedge-verify-codegen-"${TIMESTAMP}""

cleanup() {
  rm -rf "${TMPROOT}" > /dev/null
}

# Copy the current sources to a temporary directory.
# usage: save_sources api config
save_sources() {
  for i in "$@" ; do
    DIFFROOT="${PROJECT_ROOT}/${i}"
    TMP_DIFFROOT="${TMPROOT}/${i}"
    echo "copying ${DIFFROOT} to ${TMP_DIFFROOT}"
    mkdir -p "${TMP_DIFFROOT}"
    cp -a "${DIFFROOT}"/* "${TMP_DIFFROOT}"
  done
}

# Compare the generated code with the code stored in the temporary directory by save_sources.
# usage: diff_sources api config
diff_sources() {
  for i in "$@" ; do
    DIFFROOT="${PROJECT_ROOT}/${i}"
    TMP_DIFFROOT="${TMPROOT}/${i}"
    echo "diffing ${DIFFROOT} against freshly generated codegen"
    ret=0
    diff -x '.*' -Naupr "${DIFFROOT}" "${TMP_DIFFROOT}" || ret=$?
    cp -a "${TMP_DIFFROOT}"/* "${DIFFROOT}"
    if [[ $ret -eq 0 ]]
    then
      echo "${DIFFROOT} up to date."
    else
      echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
      echo "${DIFFROOT} is out of date"
      echo "Please run 'make generate' / 'make manifest'"
      echo "If that does resolve the diff your setup might contain "
      echo "outdated dependencies"
      echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
      exit 1
    fi
  done
}

# Use make commands to generate code
# usage: generate_code generate manifests
generate_code() {
  for i in "$@" ; do
    GENERATION_COMMAND="make -C "${PROJECT_ROOT}" ${i}"
    ${GENERATION_COMMAND}
    if [[ $? != "0" ]]; then
        echo "${GENERATION_COMMAND} failed"
        return 1
    fi
  done
}

# Copy back the saved sources from temporary directory.
# usage: restore_source api config
restore_source() {
  echo "Restoring the sorce files"
  for i in "$@" ; do
    DIFFROOT="${PROJECT_ROOT}/${i}"
    TMP_DIFFROOT="${TMPROOT}/${i}"
    echo "copying ${TMP_DIFFROOT} to ${DIFFROOT}"
    cp -a "${TMP_DIFFROOT}"/* "${DIFFROOT}"
  done
  echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
  echo "Restore complete, please check the code and try again"
  echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
  exit 1
}

main() {
  
  # Clean up the temporary directory after this script.
  trap cleanup EXIT SIGINT

  # Copy the current sources to a temporary directory.
  save_sources api config

  # Run the code generators
  generate_code generate manifests || restore_source api config
  
  # Compare the generated code with the code stored in the temporary directory by save_sources.
  diff_sources api config
}

main
