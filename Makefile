.PHONY: run build test vet tidy up down clean-db migrate logs ps

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

up: ## postgres → liquibase → app 전체 기동
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
