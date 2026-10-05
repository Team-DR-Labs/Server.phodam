#!/usr/bin/env bash
# 로컬 DB 전용 Liquibase 래퍼. 로컬은 draft,committed 를 모두 적용한다.
#   사용 예: ./lb-local.sh update --contexts="draft,committed"
#           ./lb-local.sh rollback-count 1
# 로컬 liquibase CLI 가 없으면 프로젝트 루트에서 `make migrate`(docker) 를 사용한다.
set -euo pipefail

cd "$(dirname "$0")"

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-phodam}"
DB_USER="${DB_USER:-phodam}"

if [[ -z "${DB_PASSWORD:-}" ]]; then
    if [[ ! -f .passwd ]]; then
        echo "error: .passwd 가 없습니다. .passwd.template 을 복사해 LOCAL_PASSWORD 를 채우세요." >&2
        exit 1
    fi
    DB_PASSWORD="$(grep '^LOCAL_PASSWORD=' .passwd | cut -d= -f2-)"
fi

if [[ $# -eq 0 ]]; then
    echo "usage: $0 <liquibase command> [args...]" >&2
    exit 1
fi

liquibase \
    --changelog-file=db-changelog-master.xml \
    --url="jdbc:postgresql://${DB_HOST}:${DB_PORT}/${DB_NAME}" \
    --username="${DB_USER}" \
    --password="${DB_PASSWORD}" \
    "$@"
