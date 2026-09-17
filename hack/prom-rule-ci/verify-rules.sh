#!/usr/bin/env bash
#
# This file is part of the KubeVirt project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Copyright 2020 Red Hat, Inc.
#

set -euo pipefail

CONTAINER_TOOL="${CONTAINER_TOOL:-docker}"
PROM_IMAGE="quay.io/prometheus/prometheus:v2.44.0"

if ! command -v "${CONTAINER_TOOL}" >/dev/null 2>&1; then
    echo "ERROR: ${CONTAINER_TOOL} is required to verify Prometheus rules"
    exit 1
fi

cleanup() {
    local cleanup_files=("${@:?}")
    for file in "${cleanup_files[@]}"; do
        rm -f "$file"
    done
}

lint() {
    local target_file="${1:?}"
    ${CONTAINER_TOOL} run --rm --entrypoint=/bin/promtool \
        -v "${target_file}:/tmp/rules.verify:ro,Z" "$PROM_IMAGE" \
        check rules /tmp/rules.verify
}

unit_test() {
    local target_file="${1:?}"
    local tests_file="${2:?}"
    ${CONTAINER_TOOL} run --rm --entrypoint=/bin/promtool \
        -v "${tests_file}:/tmp/rules.test:ro,Z" \
        -v "${target_file}:/tmp/rules.verify:ro,Z" \
        "$PROM_IMAGE" \
        test rules /tmp/rules.test
}

main() {
    local prom_spec_dumper="${1:?}"
    local tests_file="${2:?}"
    local target_file

    target_file="$(mktemp --tmpdir -u tmp.prom_rules.XXXXX)"
    # Expand target_file now: EXIT after main returns cannot see a local.
    trap "cleanup ${target_file}" RETURN EXIT INT

    "$prom_spec_dumper" "$target_file"

    echo "INFO: Rules file content:"
    cat "$target_file"
    echo

    lint "$target_file"
    unit_test "$target_file" "$tests_file"
}

main "$@"
