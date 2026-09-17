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
# Copyright 2023 Red Hat, Inc.
#

set -euo pipefail

linter_image_tag="v0.0.12"
CONTAINER_TOOL="${CONTAINER_TOOL:-docker}"

if ! command -v "${CONTAINER_TOOL}" >/dev/null 2>&1; then
    echo "ERROR: ${CONTAINER_TOOL} is required to run the metrics name linter"
    exit 1
fi

while [[ $# -gt 0 ]]; do
    case "$1" in
    --operator-name=*)
        operator_name="${1#*=}"
        shift
        ;;
    --sub-operator-name=*)
        sub_operator_name="${1#*=}"
        shift
        ;;
    --metrics-file=*)
        metrics_file="${1#*=}"
        shift
        ;;
    *)
        echo "Invalid argument: $1"
        exit 1
        ;;
    esac
done

if [[ -z "${operator_name:-}" || -z "${sub_operator_name:-}" || -z "${metrics_file:-}" ]]; then
    echo "Usage: $0 --operator-name=NAME --sub-operator-name=NAME --metrics-file=FILE"
    exit 1
fi

errors=$(${CONTAINER_TOOL} run --rm -i \
    "quay.io/kubevirt/prom-metrics-linter:${linter_image_tag}" \
    --metric-families="$(cat "$metrics_file")" \
    --operator-name="$operator_name" \
    --sub-operator-name="$sub_operator_name")

if [[ $errors != "" ]]; then
    echo "$errors"
    exit 1
fi
