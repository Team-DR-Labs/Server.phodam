--liquibase formatted sql

--changeset phodam:V261005_3_1 context:committed
--comment: 커플. 사용자당 활성 커플은 하나 (연결 해제는 미구현)
CREATE TABLE couples (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_a_id    uuid        NOT NULL REFERENCES users (id),
    user_b_id    uuid        NOT NULL REFERENCES users (id),
    status       varchar(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active')),
    connected_at timestamptz NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_couples_distinct_users CHECK (user_a_id <> user_b_id)
);
CREATE UNIQUE INDEX uq_couples_user_a_active ON couples (user_a_id) WHERE status = 'active';
CREATE UNIQUE INDEX uq_couples_user_b_active ON couples (user_b_id) WHERE status = 'active';
CREATE TRIGGER trg_couples_updated_at BEFORE UPDATE ON couples
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();
--rollback DROP TABLE IF EXISTS couples;

--changeset phodam:V261005_3_2 context:committed
--comment: 초대 코드. 24시간 유효, 1회용, 새 코드 발급 시 이전 미사용 코드 폐기
CREATE TABLE couple_invites (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    code       char(8)     NOT NULL,
    creator_id uuid        NOT NULL REFERENCES users (id),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    used_by    uuid        REFERENCES users (id),
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_couple_invites_code UNIQUE (code)
);
CREATE INDEX idx_couple_invites_creator_open ON couple_invites (creator_id)
    WHERE used_at IS NULL AND revoked_at IS NULL;
CREATE TRIGGER trg_couple_invites_updated_at BEFORE UPDATE ON couple_invites
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();
--rollback DROP TABLE IF EXISTS couple_invites;
