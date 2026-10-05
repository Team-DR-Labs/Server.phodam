.PHONY: run build test vet tidy up down clean-db migrate logs ps grant-film e2e

# .env 가 있으면 로컬 실행 시 환경 변수로 읽는다.
ifneq (,$(wildcard .env))
include .env
export
endif

run: ## 로컬에서 서버 실행 (DB 는 `make up` 으로 띄운 postgres 사용)
	go run ./cmd/server

build: ## 바이너리 빌드
	go build -o bin/server ./cmd/server

test: ## 테스트 (race 포함)
	go test -race ./...

vet: ## 정적 분석
	go vet ./...

tidy:
	go mod tidy

up: ## postgres·minio → liquibase·minio-init → app 전체 기동
	docker compose up --build -d

down: ## 컨테이너 중지
	docker compose down

clean-db: ## 컨테이너 + DB 볼륨 삭제
	docker compose down -v

migrate: ## Liquibase 마이그레이션만 다시 실행
	docker compose up -d postgres
	docker compose run --rm liquibase

logs:
	docker compose logs -f app

ps:
	docker compose ps -a

API_BASE ?= http://localhost:$(or $(APP_EXPOSE_PORT),8080)/v1
SHOTS ?= 24

# USER 는 셸의 로그인 이름과 겹치므로 명령행에서 받은 값이 UUID 인지 확인한다.
grant-film: ## 필름 수동 지급: make grant-film USER=<uuid> SHOTS=24 (ADMIN_API_KEY 필요)
	@echo "$(USER)" | grep -Eq '^[0-9a-fA-F-]{36}$$' || (echo "usage: make grant-film USER=<user uuid> SHOTS=24" && exit 1)
	@test -n "$(ADMIN_API_KEY)" || (echo "ADMIN_API_KEY 가 비어 있습니다 (.env 또는 환경 변수)" && exit 1)
	@curl -sS -X POST "$(API_BASE)/admin/users/$(USER)/film-grants" \
		-H "X-Admin-Key: $(ADMIN_API_KEY)" -H "Content-Type: application/json" \
		-d '{"shots": $(SHOTS), "reason": "make grant-film"}'
	@echo

e2e: ## 로컬 스택(make up)에 대해 E2E 시나리오 실행 (bash + curl + jq)
	API_BASE="$(API_BASE)" ADMIN_API_KEY="$(or $(ADMIN_API_KEY),local-admin-key)" ./scripts/e2e.sh
