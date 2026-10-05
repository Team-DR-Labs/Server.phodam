--liquibase formatted sql

--changeset phodam:V261005_2_1 context:committed
--comment: 사용자. film_balance 는 film_ledger 합계와 항상 같다
CREATE TABLE users (
    id           uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    nickname     varchar(20)  NOT NULL,
    film_balance integer      NOT NULL DEFAULT 0 CHECK (film_balance >= 0),
    status       varchar(20)  NOT NULL DEFAULT 'active' CHECK (status IN ('active')),
    created_at   timestamptz  NOT NULL DEFAULT now(),
    updated_at   timestamptz  NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();
--rollback DROP TABLE IF EXISTS users;

--changeset phodam:V261005_2_2 context:committed
--comment: 외부 로그인 식별자 (provider, subject)
CREATE TABLE user_identities (
    id         uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid         NOT NULL REFERENCES users (id),
    provider   varchar(20)  NOT NULL CHECK (provider IN ('apple', 'google', 'dev')),
    subject    varchar(255) NOT NULL,
    created_at timestamptz  NOT NULL DEFAULT now(),
    updated_at timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT uq_user_identities_provider_subject UNIQUE (provider, subject)
);
CREATE INDEX idx_user_identities_user_id ON user_identities (user_id);
CREATE TRIGGER trg_user_identities_updated_at BEFORE UPDATE ON user_identities
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();
--rollback DROP TABLE IF EXISTS user_identities;

--changeset phodam:V261005_2_3 context:committed
--comment: 리프레시 토큰. 원문은 저장하지 않고 SHA-256 hex 만 저장한다
CREATE TABLE refresh_tokens (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id),
    token_hash char(64)    NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_refresh_tokens_token_hash UNIQUE (token_hash)
);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id);
CREATE TRIGGER trg_refresh_tokens_updated_at BEFORE UPDATE ON refresh_tokens
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();
--rollback DROP TABLE IF EXISTS refresh_tokens;

--changeset phodam:V261005_2_4 context:committed
--comment: 필름 원장 (추가 전용). shot 의 ref_id 는 photos.id
CREATE TABLE film_ledger (
    id         uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid         NOT NULL REFERENCES users (id),
    delta      integer      NOT NULL CHECK (delta <> 0),
    reason     varchar(20)  NOT NULL CHECK (reason IN ('signup', 'shot', 'admin_grant')),
    ref_id     uuid,
    memo       varchar(100),
    created_at timestamptz  NOT NULL DEFAULT now(),
    updated_at timestamptz  NOT NULL DEFAULT now()
);
CREATE INDEX idx_film_ledger_user_id ON film_ledger (user_id, created_at);
CREATE TRIGGER trg_film_ledger_updated_at BEFORE UPDATE ON film_ledger
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();
--rollback DROP TABLE IF EXISTS film_ledger;

--changeset phodam:V261005_2_5 context:committed
--comment: FCM 기기 토큰. 토큰은 전역 유일하며 다른 사용자가 등록하면 소유자를 옮긴다
CREATE TABLE user_devices (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id),
    fcm_token  text        NOT NULL,
    platform   varchar(10) NOT NULL CHECK (platform IN ('ios', 'android')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_user_devices_fcm_token UNIQUE (fcm_token)
);
CREATE INDEX idx_user_devices_user_id ON user_devices (user_id);
CREATE TRIGGER trg_user_devices_updated_at BEFORE UPDATE ON user_devices
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();
--rollback DROP TABLE IF EXISTS user_devices;
