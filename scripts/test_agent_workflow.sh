#!/usr/bin/env bash
set -euo pipefail

: "${AGENT_API_KEY:?AGENT_API_KEY is required}"
BASE_URL="${BASE_URL:-http://127.0.0.1:8040}"
API_URL="${BASE_URL%/}/api/v1/agent"
AUTH_HEADER="Authorization: Bearer ${AGENT_API_KEY}"

command -v curl >/dev/null
command -v jq >/dev/null

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

request_json() {
  curl --fail-with-body --silent --show-error -H "$AUTH_HEADER" -H 'Content-Type: application/json' "$@"
}

kb_response="$(request_json -X POST "$API_URL/knowledge-bases" --data "{\"name\":\"Agent smoke $(date +%s)\",\"description\":\"Automated live smoke test\"}")"
kb_id="$(jq -er '.data.id' <<<"$kb_response")"

printf '# Smoke guide\n\nThe verification phrase is cobalt-orchid.\n' >"$work_dir/guide.md"
upload_response="$(curl --fail-with-body --silent --show-error -H "$AUTH_HEADER" -F "file=@$work_dir/guide.md;type=text/markdown" "$API_URL/knowledge-bases/$kb_id/documents")"
job_id="$(jq -er '.data.ingestion_job.id' <<<"$upload_response")"

for _ in $(seq 1 120); do
  job_response="$(request_json "$API_URL/ingestion-jobs/$job_id")"
  job_status="$(jq -er '.data.status' <<<"$job_response")"
  case "$job_status" in
    completed) break ;;
    failed) jq -er '.data.error_summary' <<<"$job_response" >&2; exit 1 ;;
  esac
  sleep 1
done
test "${job_status:-}" = completed

conversation_response="$(request_json -X POST "$API_URL/conversations" --data "{\"knowledge_base_id\":\"$kb_id\",\"title\":\"Smoke conversation\"}")"
conversation_id="$(jq -er '.data.id' <<<"$conversation_response")"
run_response="$(request_json -X POST "$API_URL/conversations/$conversation_id/runs" --data '{"content":"What is the verification phrase? Cite the source."}')"
run_id="$(jq -er '.data.run_id' <<<"$run_response")"

curl --fail-with-body --silent --show-error --no-buffer -H "$AUTH_HEADER" "$API_URL/runs/$run_id/events" >"$work_dir/events.sse"
grep -q '^event: answer.completed$' "$work_dir/events.sse"
first_event_id="$(awk '/^id: / {print $2; exit}' "$work_dir/events.sse")"
test -n "$first_event_id"

curl --fail-with-body --silent --show-error --no-buffer -H "$AUTH_HEADER" "$API_URL/runs/$run_id/events?after=$first_event_id" >"$work_dir/replay.sse"
grep -q '^id: ' "$work_dir/replay.sse"

final_run="$(request_json "$API_URL/runs/$run_id")"
jq -e '.data.status == "completed" and .data.total_tokens > 0 and (.data.final_message.citations | length) > 0' <<<"$final_run" >/dev/null
echo "Agent workflow smoke test passed (run_id=$run_id)"
