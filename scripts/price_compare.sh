#!/usr/bin/env bash

set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
cd "${repo_root}"

base_url="${BASE_URL:-http://127.0.0.1:8040}"
base_url="${base_url%/}"
category_url="${CATEGORY_URL:-}"
category_name="${CATEGORY_NAME:-自动价格对比}"
max_pages="${MAX_PAGES:-1}"
job_timeout="${JOB_TIMEOUT:-900}"
poll_interval="${POLL_INTERVAL:-2}"
server_start_timeout="${SERVER_START_TIMEOUT:-120}"
compare_limit="${COMPARE_LIMIT:-20}"
public_only="${PUBLIC_ONLY:-0}"
output_file="${OUTPUT_FILE:-data/price-comparison.json}"

info() {
	printf '[price-compare] %s\n' "$*" >&2
}

fail() {
	printf '[price-compare] 错误：%s\n' "$*" >&2
	exit 1
}

usage() {
	cat <<'EOF'
京东自营价格对比自动化脚本

用法：
  bash scripts/price_compare.sh login
  CATEGORY_URL='https://list.jd.com/list.html?...' bash scripts/price_compare.sh run
  bash scripts/price_compare.sh run

命令：
  run       自动启动服务、发现商品、采集公开价/结算价并输出对比表（默认）
  login     停止由本项目管理的后台服务，打开独立 Chrome 供人工登录京东
  help      显示帮助

常用环境变量：
  CATEGORY_URL          首次运行时使用的京东分类或搜索列表 HTTPS 地址
  CATEGORY_NAME         自动创建的分类名称，默认“自动价格对比”
  MAX_PAGES             分类发现页数，默认 1，范围 1-50
  COMPARE_LIMIT         最多输出多少个商品，默认 20，范围 1-100
  JOB_TIMEOUT           单个异步任务最长等待秒数，默认 900
  POLL_INTERVAL         任务轮询间隔秒数，默认 2
  OUTPUT_FILE           JSON 结果文件，默认 data/price-comparison.json
  BASE_URL              服务地址，默认 http://127.0.0.1:8040
  PUBLIC_ONLY=1         只采集公开价，不要求京东登录，不执行结算采样

说明：
  京东账号、密码、短信码、二维码和验证码必须由本人在 Chrome 中处理。
  脚本不会读取、保存或绕过这些验证信息。
EOF
}

require_command() {
	command -v "$1" >/dev/null 2>&1 || fail "缺少命令 $1"
}

require_positive_integer() {
	local name="$1"
	local value="$2"
	local maximum="$3"
	case "${value}" in
		''|*[!0-9]*) fail "${name} 必须是 1-${maximum} 的整数" ;;
	esac
	if ((value < 1 || value > maximum)); then
		fail "${name} 必须是 1-${maximum} 的整数"
	fi
}

normalize_category_url() {
	local value="$1"
	value="$(printf '%s' "${value}" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
	case "${value}" in
		\[*\]\(https://*\))
			value="${value#*](}"
			value="${value%)}"
			info '检测到 Markdown 链接，已自动提取其中的京东 URL'
			;;
		\<https://*\>)
			value="${value#<}"
			value="${value%>}"
			info '检测到尖括号链接，已自动提取其中的京东 URL'
			;;
	esac
	printf '%s\n' "${value}"
}

api_request() {
	local method="$1"
	local path="$2"
	local body="${3:-}"
	local response
	local curl_args=(
		--fail-with-body
		--silent
		--show-error
		--connect-timeout 10
		--max-time 120
		--request "${method}"
		--header 'Accept: application/json'
	)

	if [[ -n "${body}" ]]; then
		curl_args+=(--header 'Content-Type: application/json' --data "${body}")
	fi

	if ! response="$(curl "${curl_args[@]}" "${base_url}${path}")"; then
		if [[ -n "${response}" ]]; then
			if printf '%s\n' "${response}" | jq -e . >/dev/null 2>&1; then
				printf '[price-compare] 接口错误：%s %s：%s\n' \
					"${method}" \
					"${path}" \
					"$(printf '%s\n' "${response}" | jq -r '.error.detail // .error.status // "未知错误"')" >&2
				printf '%s\n' "${response}" | jq '.error // .' >&2
			else
				printf '%s\n' "${response}" >&2
			fi
		fi
		return 1
	fi

	if ! printf '%s\n' "${response}" | jq -e . >/dev/null 2>&1; then
		printf '[price-compare] 接口没有返回有效 JSON：%s\n' "${base_url}${path}" >&2
		printf '%s\n' "${response}" >&2
		return 1
	fi

	if ! printf '%s\n' "${response}" | jq -e '.success == true' >/dev/null; then
		printf '[price-compare] 接口调用失败：%s %s\n' "${method}" "${path}" >&2
		printf '%s\n' "${response}" | jq '.error // .' >&2
		return 1
	fi

	printf '%s\n' "${response}"
}

service_healthy() {
	local response
	response="$(curl --silent --show-error --connect-timeout 2 --max-time 3 "${base_url}/health" 2>/dev/null)" || return 1
	printf '%s\n' "${response}" | jq -e '.success == true' >/dev/null 2>&1
}

wait_for_service() {
	local started_at
	started_at="$(date +%s)"
	while ! service_healthy; do
		if (( $(date +%s) - started_at >= server_start_timeout )); then
			if [[ -f ginadmin.log ]]; then
				info '最近的服务日志：'
				tail -n 40 ginadmin.log >&2 || true
			fi
			fail "服务在 ${server_start_timeout} 秒内没有就绪"
		fi
		sleep 1
	done
}

ensure_service() {
	if service_healthy; then
		info "服务已就绪：${base_url}"
		return
	fi

	require_command go
	require_command make
	info '服务未运行，正在构建并以后台模式启动'
	make serve-d
	wait_for_service
	info "服务已启动：${base_url}"
}

run_login() {
	require_command go
	if service_healthy; then
		if [[ -f ginadmin.lock && -x ./ginadmin ]]; then
			info '正在停止由本项目管理的后台服务，以释放京东 Chrome Profile'
			./ginadmin stop
			local started_at
			started_at="$(date +%s)"
			while service_healthy; do
				if (( $(date +%s) - started_at >= 15 )); then
					fail '后台服务停止超时'
				fi
				sleep 1
			done
		else
			fail '检测到服务正在运行，但没有可用的 ginadmin.lock。请在启动服务的终端按 Ctrl+C，然后重新执行 make jd-login'
		fi
	fi

	info '即将打开独立 Chrome；请本人完成京东登录和可能出现的安全验证'
	go run main.go jd-login -d configs -c dev
	info '登录流程完成。现在可以执行 make price-compare'
}

ensure_session() {
	local response
	local authenticated
	local captcha_blocked
	response="$(api_request GET '/api/v1/shop/session')" || fail '无法读取京东会话状态'
	authenticated="$(printf '%s\n' "${response}" | jq -r '.data.authenticated // false')"
	captcha_blocked="$(printf '%s\n' "${response}" | jq -r '.data.captcha_blocked // false')"

	if [[ "${captcha_blocked}" == 'true' ]]; then
		fail '京东要求人工安全验证。请先执行 make jd-login，在 Chrome 中完成验证后重试'
	fi
	if [[ "${authenticated}" != 'true' ]]; then
		fail '京东尚未登录。请先执行 make jd-login，登录成功后再执行 make price-compare'
	fi
	info '京东会话有效'
}

ensure_category() {
	local response
	local existing_id
	local enabled_count
	local body
	category_url="$(normalize_category_url "${category_url}")"
	if [[ -n "${category_url}" && "${category_url}" != https://* ]]; then
		fail 'CATEGORY_URL 必须是纯 HTTPS 地址；也可以粘贴 Markdown 链接，脚本会自动提取其中的地址'
	fi
	response="$(api_request GET '/api/v1/shop/categories?current=1&pageSize=100')" || fail '无法查询监控分类'

	if [[ -n "${category_url}" ]]; then
		existing_id="$(printf '%s\n' "${response}" | jq -r --arg url "${category_url}" '.data[]? | select(.source_url == $url) | .id' | head -n 1)"
		if [[ -n "${existing_id}" ]]; then
			info "分类已存在，不重复创建：${category_name} (${existing_id})"
			return
		fi

		body="$(jq -n \
			--arg name "${category_name}" \
			--arg url "${category_url}" \
			--argjson maxPages "${max_pages}" \
			'{name: $name, source_url: $url, status: "enabled", max_pages: $maxPages}')"
		response="$(api_request POST '/api/v1/shop/categories' "${body}")" || fail '创建监控分类失败'
		info "已创建分类：$(printf '%s\n' "${response}" | jq -r '.data.name')"
		return
	fi

	enabled_count="$(printf '%s\n' "${response}" | jq '[.data[]? | select(.status == "enabled")] | length')"
	if ((enabled_count == 0)); then
		fail "当前没有启用的分类。首次运行请设置 CATEGORY_URL，例如：CATEGORY_URL='https://list.jd.com/list.html?...' make price-compare"
	fi
	info "使用已有的 ${enabled_count} 个启用分类"
}

wait_for_job() {
	local job_type="$1"
	local job_id="$2"
	local started_at
	local response
	local job
	local status
	started_at="$(date +%s)"

	while true; do
		response="$(api_request GET "/api/v1/shop/jobs?current=1&pageSize=100&type=${job_type}")" || fail "查询任务 ${job_type} 失败"
		job="$(printf '%s\n' "${response}" | jq -c --arg id "${job_id}" '.data[]? | select(.id == $id)' | head -n 1)"
		if [[ -n "${job}" ]]; then
			status="$(printf '%s\n' "${job}" | jq -r '.status')"
			case "${status}" in
				succeeded)
				info "任务完成：${job_type}，扫描 $(printf '%s\n' "${job}" | jq -r '.scanned_count')，成功 $(printf '%s\n' "${job}" | jq -r '.success_count')，失败 $(printf '%s\n' "${job}" | jq -r '.failure_count')"
				return
				;;
				failed|skipped)
					printf '%s\n' "${job}" | jq . >&2
					fail "任务 ${job_type} 结束，状态为 ${status}"
					;;
			esac
		fi

		if (( $(date +%s) - started_at >= job_timeout )); then
			fail "等待任务 ${job_type} 超过 ${job_timeout} 秒"
		fi
		sleep "${poll_interval}"
	done
}

run_job() {
	local job_type="$1"
	local response
	local job_id
	info "正在触发任务：${job_type}"
	response="$(api_request POST "/api/v1/shop/jobs/${job_type}/run")" || fail "触发任务 ${job_type} 失败"
	job_id="$(printf '%s\n' "${response}" | jq -r '.data.id // empty')"
	[[ -n "${job_id}" ]] || fail "任务 ${job_type} 没有返回任务 ID"
	wait_for_job "${job_type}" "${job_id}"
}

query_products() {
	api_request GET "/api/v1/shop/products?current=1&pageSize=${compare_limit}"
}

ensure_products_found() {
	local response="$1"
	local total
	total="$(printf '%s\n' "${response}" | jq -r '.total // 0')"
	if ((total == 0)); then
		fail '商品发现完成，但没有找到京东自营商品。桌面搜索可能尚未登录；请执行 make jd-login，完成移动端和桌面端登录后重试'
	fi
	info "已发现 ${total} 个商品；本次最多输出前 ${compare_limit} 个"
}

build_comparison() {
	local products_response="$1"
	local temp_dir
	local result_lines
	local table_file
	local product_id
	local detail
	local result
	local output_path

	temp_dir="$(mktemp -d "${TMPDIR:-/tmp}/ginadmin-price-compare.XXXXXX")"
	result_lines="${temp_dir}/results.jsonl"
	table_file="${temp_dir}/comparison.tsv"
	: >"${result_lines}"

	while IFS= read -r product_id; do
		[[ -n "${product_id}" ]] || continue
		detail="$(api_request GET "/api/v1/shop/products/${product_id}")" || fail "查询商品 ${product_id} 详情失败"
		result="$(printf '%s\n' "${detail}" | jq -c '
			.data
			| (.latest_public_price.price_fen // null) as $public
			| (.latest_checkout_price.price_fen // null) as $checkout
			| {
				id,
				sku,
				name,
				public_price_yuan: (.latest_public_price.price_yuan // null),
				checkout_price_yuan: (.latest_checkout_price.price_yuan // null),
				checkout_average_30d_yuan: (if $checkout == null then null else .checkout_average_yuan end),
				difference_yuan: (if $public != null and $checkout != null then (($checkout - $public) / 100) else null end),
				checkout_vs_public_percent: (
					if $public != null and $public > 0 and $checkout != null
					then (((($checkout - $public) * 10000 / $public) | round) / 100)
					else null
					end
				),
				alert_state,
				canonical_url
			}')"
		printf '%s\n' "${result}" >>"${result_lines}"
	done < <(printf '%s\n' "${products_response}" | jq -r '.data[]?.id')

	mkdir -p "$(dirname "${output_file}")"
	jq -s '.' "${result_lines}" >"${output_file}"

	jq -r '
		(["SKU", "商品", "公开价", "结算价", "30日结算均价", "结算减公开", "结算相对公开", "状态"] | @tsv),
		(.[] | [
			.sku,
			.name,
			(.public_price_yuan // "-"),
			(.checkout_price_yuan // "-"),
			(.checkout_average_30d_yuan // "-"),
			(if .difference_yuan == null then "-" else ((.difference_yuan | tostring) + " 元") end),
			(if .checkout_vs_public_percent == null then "-" else ((.checkout_vs_public_percent | tostring) + "%") end),
			.alert_state
		] | @tsv)' "${output_file}" >"${table_file}"

	printf '\n价格对比结果：\n\n'
	if command -v column >/dev/null 2>&1; then
		column -t -s $'\t' "${table_file}"
	else
		cat "${table_file}"
	fi

	case "${output_file}" in
		/*) output_path="${output_file}" ;;
		*) output_path="${repo_root}/${output_file}" ;;
	esac
	printf '\n完整 JSON：%s\n' "${output_path}"

	if [[ -n "${temp_dir}" && -d "${temp_dir}" ]]; then
		rm -rf -- "${temp_dir}"
	fi
}

run_comparison() {
	local products_response
	require_command curl
	require_command jq
	require_positive_integer MAX_PAGES "${max_pages}" 50
	require_positive_integer JOB_TIMEOUT "${job_timeout}" 86400
	require_positive_integer POLL_INTERVAL "${poll_interval}" 300
	require_positive_integer SERVER_START_TIMEOUT "${server_start_timeout}" 3600
	require_positive_integer COMPARE_LIMIT "${compare_limit}" 100
	if [[ "${public_only}" != '0' && "${public_only}" != '1' ]]; then
		fail 'PUBLIC_ONLY 只能是 0 或 1'
	fi

	ensure_service
	if [[ "${public_only}" != '1' ]]; then
		ensure_session
	else
		info 'PUBLIC_ONLY=1：跳过京东登录检查和结算采样'
	fi
	ensure_category

	run_job discover
	products_response="$(query_products)" || fail '查询商品失败'
	ensure_products_found "${products_response}"

	run_job public-scan
	if [[ "${public_only}" != '1' ]]; then
		run_job checkout-sample
	fi

	products_response="$(query_products)" || fail '查询商品失败'
	build_comparison "${products_response}"
}

main() {
	local command="${1:-run}"
	case "${command}" in
		run) run_comparison ;;
		login) run_login ;;
		help|-h|--help) usage ;;
		*) usage >&2; fail "不支持的命令：${command}" ;;
	esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
	main "$@"
fi
