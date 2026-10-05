--liquibase formatted sql

--changeset phodam:V261005_5_1 context:committed
--comment: 샷(사진) 레코드. 객체는 temp/permanent 버킷에 있고 DB 에는 키와 상태만 둔다
CREATE TABLE photos (
    id                uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    date_id           uuid         NOT NULL,
    owner_id          uuid         NOT NULL,
    status            varchar(20)  NOT NULL CHECK (status IN ('reserved', 'uploaded', 'archived', 'received', 'deleted')),
    is_representative boolean      NOT NULL DEFAULT false,
    temp_key          varchar(255) NOT NULL,
    permanent_key     varchar(255),
    size_bytes        bigint,
    uploaded_at       timestamptz,
    received_at       timestamptz,
    deleted_at        timestamptz,
    temp_purged_at    timestamptz,
    created_at        timestamptz  NOT NULL DEFAULT now(),
    updated_at        timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT fk_photos_participant FOREIGN KEY (date_id, owner_id)
        REFERENCES date_participants (date_id, user_id)
);
CREATE INDEX idx_photos_date_owner ON photos (date_id, owner_id, created_at);
CREATE INDEX idx_photos_live ON photos (date_id) WHERE status IN ('reserved', 'uploaded');
CREATE INDEX idx_photos_archived_temp ON photos (id) WHERE status = 'archived' AND temp_purged_at IS NULL;
CREATE TRIGGER trg_photos_updated_at BEFORE UPDATE ON photos
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();
--rollback DROP TABLE IF EXISTS photos;

--changeset phodam:V261005_5_2 context:committed
--comment: 참여자의 대표 사진 참조
ALTER TABLE date_participants
    ADD CONSTRAINT fk_date_participants_representative_photo
    FOREIGN KEY (representative_photo_id) REFERENCES photos (id);
--rollback ALTER TABLE date_participants DROP CONSTRAINT IF EXISTS fk_date_participants_representative_photo;
