BEGIN;

-- Actual yield recorded at harvest time
ALTER TABLE crop_batch ADD COLUMN IF NOT EXISTS actual_yield_kg DECIMAL(12,2);

-- Harvest recording audit trail: every harvest entry/correction is kept,
-- including which trace codes were affected by a harvest-date change.
CREATE TABLE IF NOT EXISTS harvest_record (
    id BIGSERIAL PRIMARY KEY,
    batch_id BIGINT NOT NULL REFERENCES crop_batch(id),
    harvest_date DATE NOT NULL,
    actual_yield_kg DECIMAL(12,2) NOT NULL DEFAULT 0,
    previous_harvest_date DATE,
    affected_code_count INT NOT NULL DEFAULT 0,
    affected_codes JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_harvest_record_batch ON harvest_record(batch_id);

COMMIT;
