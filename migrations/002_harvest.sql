BEGIN;

-- 采收补录：实际产量（区别于创建批次时的预计产量 expected_yield_kg）
ALTER TABLE crop_batch ADD COLUMN IF NOT EXISTS actual_yield_kg DECIMAL(12,2);

-- 采收日期变更审计：已发码批次修改采收日期时写入，用于追溯影响了哪些码
-- 受影响码 = 变更时刻该批次已生成的全部码（seq 从 1 连续递增，故记 count + max_seq 即可精确定位）
CREATE TABLE IF NOT EXISTS harvest_change (
    id BIGSERIAL PRIMARY KEY,
    batch_id BIGINT NOT NULL REFERENCES crop_batch(id),
    old_harvest_date DATE,
    new_harvest_date DATE NOT NULL,
    affected_code_count INT NOT NULL DEFAULT 0,
    affected_max_seq INT NOT NULL DEFAULT 0,
    reason VARCHAR(255),
    changed_by VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_harvest_change_batch ON harvest_change(batch_id);

COMMIT;
