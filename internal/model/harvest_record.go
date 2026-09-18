package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// StringSlice 以 jsonb 存储的字符串数组（同 StringMap 的 Value/Scan 模式）。
type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return json.Marshal([]string{})
	}
	return json.Marshal(s)
}

func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = []string{}
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, s)
}

// HarvestRecord 采收补录审计记录：每次补录/更正采收信息都会留痕，
// 已发码批次修改采收日期时，affected_codes 快照保存受影响的溯源码清单。
type HarvestRecord struct {
	ID                  int64       `db:"id" json:"id"`
	BatchID             int64       `db:"batch_id" json:"batch_id"`
	HarvestDate         time.Time   `db:"harvest_date" json:"harvest_date"`
	ActualYieldKg       float64     `db:"actual_yield_kg" json:"actual_yield_kg"`
	PreviousHarvestDate *time.Time  `db:"previous_harvest_date" json:"previous_harvest_date,omitempty"`
	AffectedCodeCount   int         `db:"affected_code_count" json:"affected_code_count"`
	AffectedCodes       StringSlice `db:"affected_codes" json:"affected_codes"`
	CreatedAt           time.Time   `db:"created_at" json:"created_at"`
}
