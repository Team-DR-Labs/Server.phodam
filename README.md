# Server.phodam

포담(함께 담는 사진일기) API 서버. Go · Gin · GORM · PostgreSQL · MinIO(S3 호환) · FCM.

| 문서 | 내용 |
|---|---|
| [`docs/mvp-policy.md`](docs/mvp-policy.md) | 도메인 정책: 상태 머신, 공개 규칙, 타이머, 워커, 에러 코드 |
| [`docs/openapi.yaml`](docs/openapi.yaml) | API 계약. 정본은 Apidog 프로젝트이고, 이 파일은 동기화본이다 |
| `docs/ARCHITECTURE.md` | 헥사고날 구조와 새 도메인을 추가하는 순서 |

## 빠른 시작

```bash
cp .env.example .env
make up            # postgres·minio → liquibase·버킷 생성 → app
make e2e           # 로컬 스택 E2E 시나리오 (bash + curl + jq)
```

| 주소 | 용도 |
|---|---|
| http://localhost:8080/v1 | API |
| http://localhost:8080/health/ready | DB 연결 포함 상태 점검 |
| http://localhost:9001 | MinIO 콘솔 (`phodam` / `phodam-secret`) |

### 개발용 로그인

`APP_ENV=local`일 때만 열린다. `APP_ENV`를 지정하지 않으면 production으로 동작하므로 이 라우트는 열리지 않는다.

```bash
curl -X POST localhost:8080/v1/auth/dev \
  -H 'content-type: application/json' \
  -d '{"dev_id":"alice","nickname":"앨리스"}'
```

- 같은 `dev_id`로 다시 로그인하면 같은 사용자다.
- 첫 로그인에서 필름 24장을 지급한다.

## 명령

| 명령 | 동작 |
|---|---|
| `make up` / `make down` | 전체 스택 기동 / 중지 |
| `make clean-db` | 컨테이너와 DB·MinIO 볼륨 삭제 |
| `make migrate` | Liquibase 마이그레이션만 다시 실행 |
| `make run` | 앱만 로컬에서 실행 (`.env` 사용, DB·MinIO는 `make up`으로 띄운 것을 씀) |
| `make test` / `make vet` | `go test -race ./...` / `go vet ./...` |
| `make e2e` | 실행 중인 로컬 스택에 대해 E2E 실행 |
| `make grant-film USER=<uuid> SHOTS=24` | 테스트용 필름 수동 지급 (`ADMIN_API_KEY` 필요) |
| `make logs` / `make ps` | 앱 로그 확인 / 컨테이너 상태 |

## 환경 변수

전체 목록과 설명은 `.env.example`에 있다. 자주 바꾸는 값은 다음과 같다.

| 변수 | 설명 |
|---|---|
| `APP_ENV` | `local`이면 dev 로그인을 연다. 지정하지 않으면 production이다 |
| `JWT_SECRET` | 액세스 토큰 서명 키(32바이트 이상). 운영에서는 반드시 교체한다 |
| `APPLE_CLIENT_IDS` | Apple ID 토큰의 `aud`, 즉 앱 번들 ID `com.drlabs.podam` |
| `GOOGLE_CLIENT_IDS` | Google ID 토큰의 `aud`(Web·iOS OAuth 클라이언트 ID, 콤마 구분) |
| `STORAGE_ENDPOINT` | 서버가 MinIO에 접근하는 주소 |
| `STORAGE_PUBLIC_ENDPOINT` | presigned URL 서명에 쓰는 **앱이 실제로 접속하는 주소** (아래 참고) |
| `ADMIN_API_KEY` | 관리자 API의 `X-Admin-Key`. 비어 있으면 관리자 라우트를 등록하지 않는다 |
| `FCM_CREDENTIALS_FILE` | FCM 서비스 계정 JSON 경로. 비어 있으면 푸시를 로그로만 출력한다 |
| `WORKER_INTERVAL` | 만료·수령 기한·임박 알림·정리 워커 주기 (기본 `1m`) |

### presigned URL 호스트

presigned URL의 서명에는 Host가 들어간다. 그래서 `STORAGE_PUBLIC_ENDPOINT`는 앱이 접속하는 호스트와 정확히 같아야 하고, 다르면 업로드가 403으로 실패한다.

| 클라이언트 | `STORAGE_PUBLIC_ENDPOINT` |
|---|---|
| iOS 시뮬레이터 | `localhost:9000` (기본값) |
| Android 실기기·에뮬레이터 + `adb reverse tcp:8080 tcp:8080`, `adb reverse tcp:9000 tcp:9000` | `localhost:9000` |
| Android 에뮬레이터 (adb reverse 없이) | `10.0.2.2:9000` |
| 같은 Wi-Fi의 실기기 | `<Mac LAN IP>:9000` |

업로드할 때는 응답의 `headers`(`Content-Type: image/jpeg`)를 그대로 보내야 한다.

### FCM 푸시 (로컬)

서비스 계정 키는 레포 밖 `~/.secrets/phodam/firebase-adminsdk.json`에 둔다. 컨테이너에는 커밋하지 않는 `docker-compose.override.yml`로 마운트한다.

```yaml
# docker-compose.override.yml (gitignore 대상)
services:
  app:
    volumes:
      - ${HOME}/.secrets/phodam/firebase-adminsdk.json:/run/secrets/fcm.json:ro
```

`.env`에 `FCM_CREDENTIALS_FILE=/run/secrets/fcm.json`을 넣고 `make up` 한다.

- Android 알림은 채널 `podam_default`, 우선순위 high로 보낸다.
- 토큰별 발송 결과는 `fcm sent`, `fcm token send failed` 로그로 남는다.

## 구조

헥사고날(Ports & Adapters) 구조다. 자세한 내용은 `docs/ARCHITECTURE.md`를 본다.

```
cmd/server/                 진입점과 조립(wire.go)
internal/
  config/                   환경 변수 로드·검증
  domain/                   user · couple · dating · photo · diary · push · apperr · health (순수 Go)
  application/
    port/in, port/out       유스케이스와 인프라 인터페이스
    service/                유스케이스 구현 (fake 포트로 단위 테스트)
  adapter/
    in/http                 Gin 핸들러 (/v1 그룹, 인증 미들웨어, 에러 코드 매핑)
    in/scheduler            주기 워커
    out/persistence/postgres  GORM 리포지토리
    out/storage             MinIO (presigned PUT/GET, 복사, 삭제)
    out/push                FCM / 로그 출력
    out/idtoken             Apple·Google JWKS 검증
    out/token, out/clock    자체 JWT 발급, 시계
db/migrations/              Liquibase (V{YYMMDD}_{N}__{module}_{desc}.sql)
scripts/e2e.sh              로컬 스택 E2E
```

## 규칙

- 스키마는 Liquibase가 전담한다. 앱은 `AutoMigrate`를 쓰지 않는다.
  - 로컬은 `draft,committed`를 모두 적용하고, 공유 DB에는 `committed`만 적용한다.
- 시간은 모두 UTC `timestamptz`로 다루고, 시간 의존 로직은 `Clock` 포트로 주입해 테스트한다.
  - 일기의 `local_date`만 Asia/Seoul 기준이다.
- 에러 응답은 `{code, message}`이고, 클라이언트는 `code`로만 분기한다.
  - 남의 리소스는 존재를 숨기기 위해 404 `NOT_FOUND`로 응답한다.
- API를 바꿀 때는 Apidog 계약을 먼저 고친 뒤 `docs/openapi.yaml`을 동기화하고 구현한다.
