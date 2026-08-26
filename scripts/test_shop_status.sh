#!/usr/bin/env bash

set -euo pipefail

base_url="${BASE_URL:-http://127.0.0.1:8040}"
access_token="${ACCESS_TOKEN:-}"

if [[ -z "${access_token}" ]]; then
	printf '%s\n' '请先设置 ACCESS_TOKEN，例如：ACCESS_TOKEN="<token>" bash scripts/test_shop_status.sh' >&2
	exit 2
fi

curl --fail-with-body --silent --show-error --request GET \
	"${base_url}/api/v1/shop/status" \
	--header "Accept: application/json" \
	--header "Authorization: Bearer ${access_token}"
printf '\n'
