--liquibase formatted sql

--changeset phodam:V261005_4_1 context:committed
--comment: 데이트 테마 (마이그레이션 시드)
CREATE TABLE themes (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    title      varchar(40) NOT NULL,
    sort_order integer     NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_themes_title UNIQUE (title)
);
CREATE TRIGGER trg_themes_updated_at BEFORE UPDATE ON themes
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();
--rollback DROP TABLE IF EXISTS themes;

--changeset phodam:V261005_4_2 context:committed
--comment: 테마별 촬영 주제
CREATE TABLE topics (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    theme_id   uuid        NOT NULL REFERENCES themes (id),
    title      varchar(60) NOT NULL,
    sort_order integer     NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_topics_theme_title UNIQUE (theme_id, title)
);
CREATE TRIGGER trg_topics_updated_at BEFORE UPDATE ON topics
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();
--rollback DROP TABLE IF EXISTS topics;

--changeset phodam:V261005_4_3 context:committed
--comment: 데이트. 커플당 in_progress 는 하나 (partial unique)
CREATE TABLE dates (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    couple_id   uuid        NOT NULL REFERENCES couples (id),
    theme_id    uuid        NOT NULL REFERENCES themes (id),
    status      varchar(20) NOT NULL CHECK (status IN ('in_progress', 'revealed', 'expired')),
    started_by  uuid        NOT NULL REFERENCES users (id),
    started_at  timestamptz NOT NULL,
    deadline_at timestamptz NOT NULL,
    revealed_at timestamptz,
    expired_at  timestamptz,
    reminded_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_dates_couple_in_progress ON dates (couple_id) WHERE status = 'in_progress';
CREATE INDEX idx_dates_couple_started ON dates (couple_id, started_at DESC);
CREATE INDEX idx_dates_in_progress_deadline ON dates (deadline_at) WHERE status = 'in_progress';
CREATE TRIGGER trg_dates_updated_at BEFORE UPDATE ON dates
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();
--rollback DROP TABLE IF EXISTS dates;

--changeset phodam:V261005_4_4 context:committed
--comment: 데이트 참여자 (데이트당 2명). 상대 주제는 revealed 전까지 응답에 넣지 않는다
CREATE TABLE date_participants (
    date_id                 uuid         NOT NULL REFERENCES dates (id),
    user_id                 uuid         NOT NULL REFERENCES users (id),
    topic_id                uuid         NOT NULL REFERENCES topics (id),
    status                  varchar(20)  NOT NULL CHECK (status IN ('assigned', 'joined', 'submitted')),
    joined_at               timestamptz,
    submitted_at            timestamptz,
    receive_deadline_at     timestamptz,
    representative_photo_id uuid,
    caption                 varchar(200),
    created_at              timestamptz  NOT NULL DEFAULT now(),
    updated_at              timestamptz  NOT NULL DEFAULT now(),
    PRIMARY KEY (date_id, user_id)
);
CREATE INDEX idx_date_participants_user ON date_participants (user_id, status);
CREATE INDEX idx_date_participants_receive_deadline ON date_participants (receive_deadline_at)
    WHERE status = 'submitted';
CREATE TRIGGER trg_date_participants_updated_at BEFORE UPDATE ON date_participants
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();
--rollback DROP TABLE IF EXISTS date_participants;
