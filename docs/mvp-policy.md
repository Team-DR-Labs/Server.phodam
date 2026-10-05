# 포담 1차 MVP 도메인 정책

> 서버와 앱이 함께 따르는 동작 기준이다. 제품 배경은 `../../podam-spec.md`, API 형태는 Apidog 프로젝트(레포 동기화본 `docs/openapi.yaml`)를 따른다.
> 이 문서와 API 계약이 충돌하면 구현을 멈추고 보고한다.

## 1. 용어

| 용어 | 의미 |
|---|---|
| 데이트(date) | 커플의 촬영 단위 하나. 테마 1개, 참여자 2명 |
| 참여자(participant) | 데이트별 사용자 상태. 주제, 제출, 수령 기한을 가진다 |
| 샷(shot) | 셔터 1회. 서버에서 필름 1장을 차감하고 사진 레코드를 예약한다 |
| 대표 사진 | 제출 시 고른 한 장. 영구 버킷으로 복사해 일기에 보관한다 |
| 수령(receive) | 제출 후 내 사진을 OS 사진첩에 저장하고 서버에 저장 성공을 알리는 것 |

## 2. 시간 규칙

- 서버의 모든 시각은 UTC `timestamptz`다. 응답은 RFC 3339(UTC, `Z`)로 내보낸다.
- `deadline_at = started_at + 72h`: 두 사람 모두의 제출 마감이다.
- `receive_deadline_at = submitted_at + 7d`: 참여자별 수령 마감이다. 데이트가 만료되어도 줄어들지 않는다.
- 만료 판정은 **요청 시점에도 즉시 수행**한다. 워커가 늦게 돌더라도 `now >= deadline_at`이면 촬영과 제출을 거부한다.
- 일기 목록의 `local_date`는 `started_at`을 Asia/Seoul로 변환한 날짜(`YYYY-MM-DD`)다.
- 시간 의존 로직은 `Clock` 포트로 주입해 테스트한다.

## 3. 계정·인증

- 로그인 수단은 Apple, Google, dev 세 가지다. dev는 `APP_ENV=local`에서만 라우트를 등록한다.
- Apple과 Google ID 토큰은 각 JWKS로 서명을 검증한다. `iss`, `aud`(설정값), `exp`도 확인한다. 식별자는 `(provider, sub)`다.
- 첫 로그인이면 사용자를 만들고 **필름 24장을 지급**한다(원장 사유 `signup`).
- 닉네임은 토큰의 이름 → 요청 본문의 `nickname` → 기본값 `포담 사용자` 순으로 정한다. `PATCH /me`로 바꿀 수 있고 1~20자다.
- 액세스 토큰은 HS256 JWT로 1시간 유효하다(`sub`=user id). 리프레시 토큰은 불투명 랜덤 문자열로 30일 유효하고, DB에는 SHA-256 해시만 저장한다. 사용할 때마다 회전하며 이전 토큰은 폐기한다.
- 탈퇴는 MVP 범위가 아니다. `users.status`는 `active`로만 쓴다.

## 4. 커플

- 초대 코드: 8자. 문자 집합은 `ABCDEFGHJKMNPQRSTUVWXYZ23456789`. 24시간 유효하고 1회만 쓴다. 새 코드를 만들면 같은 사용자가 이전에 만든 미사용 코드는 무효가 된다.
- 연결 조건: 두 사용자 모두 활성 커플이 없어야 하고, 자기 코드는 쓸 수 없다.
- 커플이 없는 사용자는 데이트·촬영·일기 API를 쓸 수 없다(`COUPLE_REQUIRED`).
- **연결 해제와 재연결은 미구현이다(정책 기획 필요).** 스키마는 `couples.status`(`active`)만 둔다.

## 5. 데이트 상태 머신

```
in_progress ──(두 사람 모두 제출)──▶ revealed
     │
     └──(now ≥ deadline_at, 둘 중 한 명 이상 미제출)──▶ expired
```

참여자 상태: `assigned`(주제 배정, 아직 확인 안 함) → `joined`(주제 확인) → `submitted`

- 시작(`POST /dates`)
  - 커플당 `in_progress`는 하나만 허용한다(DB partial unique). 이미 있으면 `DATE_ALREADY_IN_PROGRESS`.
  - 테마는 커플이 아직 쓰지 않은 테마 중 랜덤으로 고른다. 모두 썼으면 직전 데이트의 테마를 뺀 전체에서 랜덤으로 고른다.
  - 테마의 주제 중 서로 다른 2개를 랜덤으로 골라 두 참여자에게 배정한다. 다시 뽑기는 없다.
  - 시작자는 즉시 `joined`, 상대는 `assigned`가 된다. 상대에게 `date_started` 푸시를 보낸다.
- 참여(`POST /dates/{id}/join`): `assigned`이면 `joined`로 바꾸고 주제를 공개한다. 이미 `joined` 이상이면 같은 응답을 다시 준다(멱등).
- 내 주제는 `joined` 이후에만 응답에 넣는다. **상대 주제는 `revealed` 전까지 절대 응답에 넣지 않는다.** 상대 정보는 닉네임과 참여자 상태만 준다.

## 6. 촬영과 필름

- 촬영 조건: 데이트가 `in_progress`이고 기한 전이며, 내가 `joined`이고 미제출 상태여야 한다.
- 샷 예약(`POST /dates/{id}/shots`)은 한 트랜잭션으로 처리한다.
  1. `UPDATE users SET film_balance = film_balance - 1 WHERE id = ? AND film_balance > 0`. 영향받은 행이 0이면 `FILM_EXHAUSTED`.
  2. `film_ledger`에 `delta=-1`, `reason=shot`, `ref_id=photo_id`를 기록한다.
  3. `photos`에 `status=reserved`, `temp_key=temp/{date_id}/{user_id}/{photo_id}.jpg`로 생성한다.
  4. temp 버킷 presigned PUT URL(15분, `Content-Type: image/jpeg`)을 돌려준다.
- 업로드 실패, 앱 종료, 사진 삭제가 있어도 **차감한 필름은 복구하지 않는다.**
- `POST /photos/{id}/upload-url`: `reserved` 사진에 업로드 URL을 다시 발급한다(데이트가 활성이고 내가 미제출일 때).
- `POST /photos/{id}/complete`: 서버가 temp 객체를 HEAD로 확인한다. 없으면 `PHOTO_NOT_UPLOADED`. 20MB를 넘으면 객체를 지우고 `PHOTO_INVALID`. 정상이면 `uploaded`로 바꾼다. 이미 `uploaded`면 멱등 처리한다.
- 필름 지급: 관리자 API는 `reason=admin_grant`, 가입 지급은 `reason=signup`이다. 잔액은 `users.film_balance`이고, 원장 합계와 항상 같아야 한다.

## 7. 제출

- 조건: 데이트가 활성이고 기한 전이며, 내가 `joined`이고 미제출이어야 한다. 대표 사진은 내 사진이면서 `uploaded` 상태여야 한다.
- 글(caption)은 선택이며 앞뒤 공백을 제거한 뒤 최대 200자(유니코드 rune)다. 빈 문자열은 null로 저장한다.
- 처리 순서
  1. 대표 사진을 temp에서 permanent(`permanent/{date_id}/{user_id}/{photo_id}.jpg`)로 서버 측 복사한다.
  2. 사진 상태를 `archived`, `is_representative=true`로 바꾸고 참여자에 `representative_photo_id`를 기록한다.
  3. 참여자를 `submitted`로 바꾸고 `receive_deadline_at`을 기록한다.
  4. 이 데이트에서 아직 `reserved`인 내 사진은 `deleted`로 바꾼다(업로드되지 않은 샷).
  5. temp의 대표 사진 사본을 지운다. 실패해도 응답은 성공으로 보내고 워커가 정리한다.
  6. 상대도 제출했으면 데이트를 `revealed`로 바꾸고 둘 다에게 `date_revealed` 푸시를 보낸다. 아니면 상대에게 `partner_submitted` 푸시를 보낸다.
- 1~4와 6의 상태 변경은 한 트랜잭션이다. 객체 복사는 트랜잭션 전에 수행하고, 트랜잭션이 실패하면 permanent 사본을 지운다.
- 제출한 내용은 수정할 수 없다. 다시 제출하면 `ALREADY_SUBMITTED`.

## 8. 사진 상태와 보관

| photos.status | 의미 | 객체 위치 |
|---|---|---|
| `reserved` | 샷 예약, 업로드 전 | 없음 또는 업로드 중 |
| `uploaded` | temp 업로드 완료 | temp |
| `archived` | 제출한 대표 사진 | permanent (temp 사본은 삭제) |
| `received` | 비대표 사진, 기기 저장 성공 ack 완료 | 삭제됨 |
| `deleted` | 만료, 기한 경과, 미업로드 정리 | 삭제됨 |

- 대표 사진 수령 ack는 `received_at`만 기록하고 상태는 `archived`로 둔다. permanent 사본은 지우지 않는다.

## 9. 사진 열람과 수령

- `GET /dates/{id}/my-photos`: 이 데이트에서 `uploaded`와 `archived` 상태인 내 사진을 presigned GET(15분)과 함께 준다. 제출 전 미리보기용이다. **앱은 이 사진의 저장·공유 기능을 제공하지 않는다.**
- `GET /dates/{id}/receivable`: 제출했고 `now < receive_deadline_at`일 때만 쓸 수 있다. 아니면 `RECEIVE_NOT_AVAILABLE`.
  - 비대표 사진은 `uploaded`인 것만 포함한다.
  - 대표 사진은 `archived`면 `received_at`과 관계없이 permanent URL로 포함한다.
- `POST /photos/{id}/received`: **기기 저장에 성공한 다음에만** 호출한다. 비대표 사진은 temp 객체를 지우고 `received`로 바꾼다. 대표 사진은 `received_at`만 기록한다. 같은 요청은 멱등 처리한다. 수령 기한이 지났으면 `RECEIVE_NOT_AVAILABLE`.
- 저장에 실패한 사진은 ack하지 않는다. 그 사진은 서버에 남고 기한 안에 다시 받을 수 있다.

## 10. 일기 열람과 공개

| 데이트 상태 | 내가 제출 | 상대 제출 | 목록 노출 | 상세 내용 |
|---|---|---|---|---|
| revealed | O | O | O (`shared`) | 두 사람의 대표 사진, 글, 주제 |
| in_progress | O | X | O (`waiting`) | 내 대표 사진, 글, 주제만 |
| expired | O | X | O (`private`) | 내 대표 사진, 글, 주제만 |
| expired / in_progress | X | 상관없음 | X | 볼 수 없음(`NOT_FOUND`) |

- 목록은 `started_at` 내림차순이고 커서는 불투명 문자열이다. 기본 20개, 최대 50개다.
- 상대의 대표 사진과 글은 `revealed`일 때만 응답에 넣는다.

## 11. 워커 (서버 프로세스 내부, 1분 주기)

1. **만료**: `status=in_progress AND deadline_at <= now`인 데이트를 `FOR UPDATE SKIP LOCKED`로 잡아 `expired`로 바꾼다. 미제출 참여자의 `reserved`와 `uploaded` 사진은 객체를 지우고 `deleted`로 바꾼다.
2. **수령 기한**: `receive_deadline_at <= now`인 참여자의 `uploaded` 비대표 사진은 객체를 지우고 `deleted`로 바꾼다. `archived`는 지우지 않는다.
3. **만료 임박 알림**: `in_progress AND deadline_at - 6h <= now AND reminded_at IS NULL`이면 미제출 참여자에게 `deadline_soon` 푸시를 보내고 `reminded_at`을 기록한다.
4. **정리**: `archived`인데 temp 사본이 남아 있으면 삭제를 다시 시도한다.

- 객체 삭제에 실패하면 상태를 바꾸지 않고 다음 주기에 다시 시도한다. 객체가 이미 없으면 성공으로 본다.

## 12. 푸시

| type | 수신자 | 시점 |
|---|---|---|
| `date_started` | 상대 | 데이트 시작 |
| `partner_submitted` | 상대 | 첫 번째 제출 |
| `date_revealed` | 두 사람 | 공동 공개 |
| `deadline_soon` | 미제출 참여자 | 마감 6시간 전 |

- data payload는 `{ "type": "...", "date_id": "..." }`다. 알림 문구는 서버가 정한다.
- `PushSender` 포트를 쓴다. FCM 자격 증명이 없으면 slog 출력 구현을 쓴다. 발송 실패는 로그만 남기고 비즈니스 트랜잭션에 영향을 주지 않는다.
- 기기 토큰은 `PUT /me/devices`로 등록한다. 같은 토큰이 다른 사용자에게 등록되면 소유자를 옮긴다. FCM이 `UNREGISTERED`를 돌려주면 그 토큰을 삭제한다.

## 13. 에러 코드

응답 본문은 `{ "code": "...", "message": "..." }`다. message는 사람용 설명이므로 앱은 code로만 분기한다.

| HTTP | code | 상황 |
|---|---|---|
| 400 | `VALIDATION_FAILED` | 요청 형식이나 길이 오류 |
| 401 | `UNAUTHORIZED` | 액세스 토큰 없음, 만료, 위조 |
| 401 | `AUTH_INVALID_ID_TOKEN` | Apple/Google 토큰 검증 실패 |
| 401 | `AUTH_INVALID_REFRESH_TOKEN` | 리프레시 토큰 없음, 만료, 폐기 |
| 403 | `FORBIDDEN` | 관리자 키 오류 |
| 403 | `COUPLE_REQUIRED` | 커플이 없음 |
| 404 | `NOT_FOUND` | 리소스 없음, 남의 리소스, 형식이 틀린 UUID 경로(존재 여부를 숨김), reserved 가 아닌 사진의 업로드 URL 재발급 |
| 409 | `COUPLE_ALREADY_CONNECTED` | 본인이나 상대가 이미 연결됨 |
| 400 | `INVITE_INVALID` | 코드 없음, 만료, 사용됨, 본인 코드 |
| 409 | `DATE_ALREADY_IN_PROGRESS` | 진행 중 데이트 있음 |
| 409 | `DATE_NOT_ACTIVE` | 만료 또는 공개된 데이트에 촬영·제출 |
| 409 | `DATE_NOT_JOINED` | 주제 확인 전 촬영·제출 |
| 409 | `ALREADY_SUBMITTED` | 이미 제출함(제출 후 촬영 포함) |
| 409 | `FILM_EXHAUSTED` | 잔여 필름 0 |
| 409 | `PHOTO_NOT_UPLOADED` | 업로드 전 사진으로 complete·제출 |
| 400 | `PHOTO_INVALID` | 크기 초과 등 |
| 409 | `RECEIVE_NOT_AVAILABLE` | 미제출이거나 수령 기한 경과 |
| 500 | `INTERNAL_ERROR` | 그 외 |
