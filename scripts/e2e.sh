#!/usr/bin/env bash
# 포담 MVP E2E 시나리오 (podam-spec.md §10).
# `make up` 으로 띄운 로컬 스택(app + postgres + minio)에 dev 계정 두 개로 요청을 보내 전 흐름을 검증한다.
# 시간 의존 규칙(72시간 만료, 7일 수령 기한)은 Clock 을 주입한 서비스·워커 테스트에서 검증한다.
#
# 사용: scripts/e2e.sh            (또는 make e2e)
# 환경: API_BASE (기본 http://localhost:8080/v1), ADMIN_API_KEY (기본 local-admin-key)
set -euo pipefail

API_BASE="${API_BASE:-http://localhost:8080/v1}"
ADMIN_API_KEY="${ADMIN_API_KEY:-local-admin-key}"
RUN_ID="$(date +%s)-$RANDOM"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

command -v jq >/dev/null || { echo "jq 가 필요합니다" >&2; exit 1; }

STATUS=""
BODY=""
STEP=0

pass() { STEP=$((STEP + 1)); printf '  ok %02d  %s\n' "$STEP" "$1"; }
fail() { printf '  FAIL    %s\n          status=%s body=%s\n' "$1" "$STATUS" "$BODY" >&2; exit 1; }

# api METHOD PATH [TOKEN] [JSON]  →  STATUS, BODY
api() {
  local method=$1 path=$2 token=${3:-} data=${4:-}
  local args=(-sS -o "$TMP/body" -w '%{http_code}' -X "$method" "$API_BASE$path" -H 'Content-Type: application/json')
  [[ -n $token ]] && args+=(-H "Authorization: Bearer $token")
  [[ -n $data ]] && args+=(-d "$data")
  STATUS=$(curl "${args[@]}")
  BODY=$(cat "$TMP/body")
}

admin() {
  local path=$1 key=$2 data=$3
  STATUS=$(curl -sS -o "$TMP/body" -w '%{http_code}' -X POST "$API_BASE$path" \
    -H 'Content-Type: application/json' -H "X-Admin-Key: $key" -d "$data")
  BODY=$(cat "$TMP/body")
}

# expect STATUS [CODE] DESCRIPTION
expect() {
  local want=$1 code=$2 desc=$3
  [[ $STATUS == "$want" ]] || fail "$desc (want HTTP $want)"
  if [[ -n $code ]]; then
    [[ $(jq -r .code <<<"$BODY") == "$code" ]] || fail "$desc (want code $code)"
  fi
  pass "$desc"
}

# check JQ_FILTER DESCRIPTION : BODY 에 대해 jq -e 가 참이어야 한다.
check() {
  jq -e "$1" <<<"$BODY" >/dev/null || fail "$2"
  pass "$2"
}

j() { jq -r "$1" <<<"$BODY"; }

# put_object URL : presigned PUT 으로 JPEG 바이트를 올린다.
put_object() {
  head -c 4096 /dev/urandom >"$TMP/photo.jpg"
  curl -sS -o /dev/null -w '%{http_code}' -X PUT -H 'Content-Type: image/jpeg' --data-binary @"$TMP/photo.jpg" "$1"
}

get_status() { curl -sS -o /dev/null -w '%{http_code}' "$1"; }

# shoot TOKEN DATE_ID : 샷 예약 → 업로드 → complete, 사진 ID 를 PHOTO 에 둔다.
shoot() {
  api POST "/dates/$2/shots" "$1"
  [[ $STATUS == 201 ]] || fail "shot reserve"
  PHOTO=$(j .photo.id)
  local url; url=$(j .upload.url)
  [[ $(put_object "$url") == 200 ]] || fail "presigned PUT"
  api POST "/photos/$PHOTO/complete" "$1"
  [[ $STATUS == 200 && $(j .status) == uploaded ]] || fail "complete upload"
}

echo "포담 E2E — $API_BASE (run $RUN_ID)"

echo "[0] 기동 확인"
STATUS=$(curl -sS -o /dev/null -w '%{http_code}' "${API_BASE%/v1}/health/ready" || true)
[[ $STATUS == 200 ]] || { echo "서버가 준비되지 않았습니다. make up 을 먼저 실행하세요." >&2; exit 1; }
pass "GET /health/ready"

echo "[1] 로그인 (dev)"
api POST /auth/dev "" "{\"dev_id\":\"alice-$RUN_ID\",\"nickname\":\"앨리스\"}"
expect 200 "" "alice 첫 로그인"
check '.is_new_user == true and .user.nickname == "앨리스"' "새 사용자 + 닉네임"
A=$(j .access_token); A_REFRESH=$(j .refresh_token); A_ID=$(j .user.id)
api POST /auth/dev "" "{\"dev_id\":\"bob-$RUN_ID\",\"nickname\":\"밥\"}"
expect 200 "" "bob 첫 로그인"
B=$(j .access_token)
api POST /auth/dev "" "{\"dev_id\":\"alice-$RUN_ID\"}"
check '.is_new_user == false' "같은 dev_id 재로그인은 같은 사용자"
api GET /me "$A"
check '.film_balance == 24 and .couple == null and .current_date == null' "가입 필름 24장"
api GET /me
expect 401 UNAUTHORIZED "토큰 없이 /me"
api PUT /me/devices "$A" "{\"fcm_token\":\"e2e-alice-$RUN_ID\",\"platform\":\"ios\"}"
expect 204 "" "alice 기기 토큰 등록"
api PUT /me/devices "$B" "{\"fcm_token\":\"e2e-bob-$RUN_ID\",\"platform\":\"android\"}"
expect 204 "" "bob 기기 토큰 등록"
api PUT /me/devices "$B" '{"fcm_token":"x","platform":"windows"}'
expect 400 VALIDATION_FAILED "잘못된 플랫폼 거부"

echo "[2] 커플 연결"
api POST /dates "$A"
expect 403 COUPLE_REQUIRED "커플 없이 데이트 시작 불가"
api POST /couple/invites "$A"
expect 201 "" "초대 코드 발급"
CODE=$(j .code)
api POST /couple/join "$A" "{\"code\":\"$CODE\"}"
expect 400 INVITE_INVALID "자기 코드로 연결 불가"
api POST /couple/join "$B" "{\"code\":\"$CODE\"}"
expect 200 "" "bob 이 코드로 연결"
check ".partner.id == \"$A_ID\"" "연결 상대는 alice"
api POST /couple/join "$B" "{\"code\":\"$CODE\"}"
expect 409 COUPLE_ALREADY_CONNECTED "이미 연결된 사용자"

echo "[3] 데이트 시작과 주제 공개 규칙"
api POST /dates "$A"
expect 201 "" "alice 데이트 시작"
check '.me.status == "joined" and .me.topic != null and .partner.status == "assigned" and .partner.topic == null' "시작자는 내 주제만, 상대 주제 비공개"
DATE=$(j .id); A_TOPIC=$(j .me.topic.id)
api POST /dates "$B"
expect 409 DATE_ALREADY_IN_PROGRESS "진행 중 데이트 중복 시작 불가"
api GET "/dates/$DATE" "$B"
check '.me.status == "assigned" and .me.topic == null and .partner.topic == null' "참여 전 bob 은 어떤 주제도 못 봄"
api POST "/dates/$DATE/shots" "$B"
expect 409 DATE_NOT_JOINED "주제 확인 전 촬영 불가"
api POST "/dates/$DATE/join" "$B"
expect 200 "" "bob 참여(주제 확인)"
check ".me.topic != null and .me.topic.id != \"$A_TOPIC\" and .partner.topic == null" "bob 은 자기 주제만, 서로 다른 주제"
B_TOPIC=$(j .me.topic.id)
api POST "/dates/$DATE/join" "$B"
check '.me.status == "joined"' "참여는 멱등"
api GET "/dates/$DATE" "$A"
check '.partner.status == "joined" and .partner.topic == null' "alice 도 bob 주제는 못 봄"

echo "[4] 촬영과 업로드"
api POST "/dates/$DATE/shots" "$A"
expect 201 "" "샷 예약"
check '.film_balance == 23 and .photo.status == "reserved" and .upload.method == "PUT" and .upload.headers["Content-Type"] == "image/jpeg"' "필름 1장 차감 + 업로드 정보"
REP=$(j .photo.id); REP_URL=$(j .upload.url)
api POST "/photos/$REP/complete" "$A"
expect 409 PHOTO_NOT_UPLOADED "업로드 전 complete 거부"
[[ $(curl -sS -o /dev/null -w '%{http_code}' -X PUT -H 'Content-Type: image/png' --data-binary 'x' "$REP_URL") == 403 ]] || fail "Content-Type 서명 불일치"
pass "서명과 다른 Content-Type 업로드 거부"
[[ $(put_object "$REP_URL") == 200 ]] || fail "presigned PUT"
pass "presigned PUT 업로드"
api POST "/photos/$REP/complete" "$A"
expect 200 "" "complete"
check '.status == "uploaded" and .uploaded_at != null' "uploaded 로 변경"
api POST "/photos/$REP/complete" "$A"
check '.status == "uploaded"' "complete 멱등"
api POST "/dates/$DATE/shots" "$A"
EXTRA=$(j .photo.id)
api POST "/photos/$EXTRA/upload-url" "$A"
expect 200 "" "업로드 URL 재발급"
[[ $(put_object "$(j .url)") == 200 ]] || fail "재발급 URL 로 PUT"
api POST "/photos/$EXTRA/complete" "$A"
check '.status == "uploaded"' "재발급 URL 로 업로드한 사진 complete"
api POST "/dates/$DATE/shots" "$A"
DANGLING=$(j .photo.id)
check '.film_balance == 21' "업로드하지 않은 샷도 필름 차감"
shoot "$B" "$DATE"; B_REP=$PHOTO
pass "bob 촬영·업로드"

echo "[5] 제출 전 상대 열람 불가"
api GET "/dates/$DATE/my-photos" "$A"
check '(.items | length) == 2 and all(.items[]; .status == "uploaded" and (.url | startswith("http")))' "내 미리보기 2장"
PREVIEW=$(j '.items[0].url')
[[ $(get_status "$PREVIEW") == 200 ]] || fail "미리보기 URL GET"
pass "미리보기 presigned GET"
api GET "/dates/$DATE/my-photos" "$B"
check "(.items | length) == 1 and .items[0].id == \"$B_REP\"" "bob 미리보기에는 bob 사진만"
api POST "/photos/$REP/complete" "$B"
expect 404 NOT_FOUND "남의 사진 접근 불가"
api GET "/diaries/$DATE" "$B"
expect 404 NOT_FOUND "제출 전에는 일기 열람 불가"
api GET "/dates/$DATE/receivable" "$A"
expect 409 RECEIVE_NOT_AVAILABLE "제출 전 수령 불가"

echo "[6] alice 제출"
api POST "/dates/$DATE/submit" "$A" "{\"photo_id\":\"$REP\",\"caption\":\"  오늘 참 따뜻했다  \"}"
expect 200 "" "대표 사진 + 글 제출"
check '.status == "in_progress" and .me.status == "submitted" and .me.receive_deadline_at != null and .partner.topic == null' "제출 후에도 상대 주제 비공개"
api POST "/dates/$DATE/submit" "$A" "{\"photo_id\":\"$EXTRA\"}"
expect 409 ALREADY_SUBMITTED "재제출(수정) 불가"
api POST "/dates/$DATE/shots" "$A"
expect 409 ALREADY_SUBMITTED "제출 후 촬영 불가"
api POST "/photos/$DANGLING/complete" "$A"
expect 404 NOT_FOUND "업로드 안 된 샷은 제출 시 삭제"

echo "[7] alice 수령, bob 은 공개 전 열람 불가"
api GET "/dates/$DATE/receivable" "$A"
expect 200 "" "제출 직후 수령 가능"
check "(.items | length) == 2 and any(.items[]; .id == \"$REP\" and .status == \"archived\" and .is_representative) and any(.items[]; .id == \"$EXTRA\" and .status == \"uploaded\")" "대표 + 비대표 사진"
EXTRA_URL=$(jq -r ".items[] | select(.id == \"$EXTRA\") | .url" <<<"$BODY")
REP_GET=$(jq -r ".items[] | select(.id == \"$REP\") | .url" <<<"$BODY")
[[ $(get_status "$EXTRA_URL") == 200 && $(get_status "$REP_GET") == 200 ]] || fail "수령 URL 다운로드"
pass "수령 URL 다운로드"
api GET "/dates/$DATE" "$B"
check '.partner.status == "submitted" and .partner.topic == null' "bob 은 alice 제출 사실만 보고 주제는 못 봄"
api GET "/diaries/$DATE" "$B"
expect 404 NOT_FOUND "bob 은 공개 전 alice 일기 못 봄"
api GET /diaries "$B"
check '(.items | length) == 0' "bob 일기 목록 비어 있음"
api GET "/diaries/$DATE" "$A"
check '.visibility == "waiting" and (.entries | length) == 1 and .entries[0].is_me and .entries[0].caption == "오늘 참 따뜻했다"' "alice 는 내 일기만(waiting), 글 공백 제거"

echo "[8] 수령 ack"
api POST "/photos/$EXTRA/received" "$A"
expect 200 "" "비대표 사진 저장 성공 ack"
check '.status == "received" and .received_at != null' "received 로 변경"
api POST "/photos/$EXTRA/received" "$A"
check '.status == "received"' "ack 멱등"
[[ $(get_status "$EXTRA_URL") == 404 ]] || fail "ack 후 temp 객체 삭제"
pass "ack 후 temp 객체 삭제 확인 (기존 URL 404)"
api POST "/photos/$REP/received" "$A"
check '.status == "archived" and .received_at != null' "대표 사진 ack 는 received_at 만 기록"
api GET "/dates/$DATE/receivable" "$A"
check "(.items | length) == 1 and .items[0].id == \"$REP\"" "대표 사진은 계속 수령 목록에 남음"
api GET "/diaries/$DATE" "$A"
[[ $(get_status "$(j '.entries[0].photo_url')") == 200 ]] || fail "대표 사진 일기 표시"
pass "대표 사진은 ack 후에도 일기에 표시"

echo "[9] bob 제출 → 공동 공개"
api POST "/dates/$DATE/submit" "$B" "{\"photo_id\":\"$B_REP\"}"
expect 200 "" "bob 제출 (글 없이)"
check ".status == \"revealed\" and .revealed_at != null and .partner.topic.id == \"$A_TOPIC\"" "revealed + 상대 주제 공개"
api GET "/diaries/$DATE" "$B"
check ".visibility == \"shared\" and (.entries | length) == 2 and .entries[0].is_me and .entries[0].caption == null and .entries[1].author.id == \"$A_ID\" and .entries[1].caption == \"오늘 참 따뜻했다\"" "상세 2개(내 것 먼저), 상대 글 공개"
for url in $(jq -r '.entries[].photo_url' <<<"$BODY"); do
  [[ $(get_status "$url") == 200 ]] || fail "공개 일기 사진"
done
pass "두 대표 사진 모두 표시"
api GET "/diaries/$DATE" "$A"
check ".entries[1].topic.id == \"$B_TOPIC\"" "alice 도 bob 주제 확인"
api GET "/diaries?limit=1" "$A"
check ".items[0].date_id == \"$DATE\" and .items[0].visibility == \"shared\" and (.items[0].local_date | test(\"^[0-9]{4}-[0-9]{2}-[0-9]{2}$\")) and .next_cursor == null" "일기 목록(shared, local_date)"
[[ $(get_status "$(j '.items[0].thumbnail_url')") == 200 ]] || fail "썸네일"
pass "썸네일 표시"
api GET /dates/current "$A"
expect 204 "" "공개 후 진행 중 데이트 없음"

echo "[10] 필름 소진과 관리자 지급"
api POST /dates "$B"
expect 201 "" "새 데이트 시작"
DATE2=$(j .id)
api POST "/dates/$DATE2/join" "$A"
expect 200 "" "alice 참여"
api GET /me "$A"
LEFT=$(j .film_balance)
for _ in $(seq 1 "$LEFT"); do
  api POST "/dates/$DATE2/shots" "$A"
  [[ $STATUS == 201 ]] || fail "필름 소진 중 샷"
done
pass "남은 필름 ${LEFT}장 소진"
api POST "/dates/$DATE2/shots" "$A"
expect 409 FILM_EXHAUSTED "필름 0 에서 촬영 거부"
api GET /me "$A"
check '.film_balance == 0' "잔량 0 (음수 아님)"
admin "/admin/users/$A_ID/film-grants" wrong-key '{"shots":24}'
expect 403 FORBIDDEN "잘못된 관리자 키"
admin "/admin/users/$A_ID/film-grants" "$ADMIN_API_KEY" '{"shots":24,"reason":"e2e"}'
expect 200 "" "관리자 필름 지급"
check ".user_id == \"$A_ID\" and .film_balance == 24" "지급 후 잔량 24"
api POST "/dates/$DATE2/shots" "$A"
expect 201 "" "지급 후 촬영 가능"

echo "[11] 토큰 갱신·로그아웃"
api POST /auth/refresh "" "{\"refresh_token\":\"$A_REFRESH\"}"
expect 200 "" "리프레시"
NEW_REFRESH=$(j .refresh_token)
api POST /auth/refresh "" "{\"refresh_token\":\"$A_REFRESH\"}"
expect 401 AUTH_INVALID_REFRESH_TOKEN "회전된 이전 토큰 거부"
api POST /auth/logout "$A" "{\"refresh_token\":\"$NEW_REFRESH\"}"
expect 204 "" "로그아웃"
api POST /auth/refresh "" "{\"refresh_token\":\"$NEW_REFRESH\"}"
expect 401 AUTH_INVALID_REFRESH_TOKEN "로그아웃한 토큰 거부"

echo
echo "ALL PASSED ($STEP checks)"
