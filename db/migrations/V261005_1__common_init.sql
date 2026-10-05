--liquibase formatted sql

--changeset phodam:V261005_1 context:committed splitStatements:false
--comment: 공통 updated_at 자동 갱신 트리거 함수 (테이블 생성 시 BEFORE UPDATE 트리거로 연결)
CREATE OR REPLACE FUNCTION fn_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
--rollback DROP FUNCTION IF EXISTS fn_set_updated_at();
