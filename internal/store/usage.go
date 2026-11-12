package store

import (
	"database/sql"
	"time"
)

type UsageEntry struct {
	UserID           int64
	ThreadID         int64
	ProviderID       int64
	ModelName        string
	Kind             string // chat | compaction
	PromptTokens     int
	CompletionTokens int
	Cost             float64
	Billable         bool
	Estimated        bool
}

func (s *Store) RecordUsage(e UsageEntry) error {
	_, err := s.DB.Exec(`INSERT INTO usage_log(user_id, thread_id, provider_id, model_name, kind,
		prompt_tokens, completion_tokens, cost, billable, estimated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.UserID, e.ThreadID, e.ProviderID, e.ModelName, e.Kind,
		e.PromptTokens, e.CompletionTokens, e.Cost, b2i(e.Billable), b2i(e.Estimated))
	return err
}

// PeriodStart returns the start (UTC) of the current quota period.
func PeriodStart(period string, now time.Time) time.Time {
	now = now.UTC()
	if period == "daily" {
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	}
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// UsedTokens sums billable tokens for the user's current period.
func (s *Store) UsedTokens(userID int64, period string) (int64, error) {
	var n int64
	err := s.DB.QueryRow(`SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0) FROM usage_log
		WHERE user_id = ? AND billable = 1 AND created_at >= ?`,
		userID, FormatTime(PeriodStart(period, time.Now()))).Scan(&n)
	return n, err
}

type Quota struct {
	Period     string
	TokenLimit int64 // 0 = unlimited
}

func (s *Store) QuotaFor(userID int64) (Quota, error) {
	q := Quota{Period: "monthly"}
	err := s.DB.QueryRow(`SELECT period, token_limit FROM quotas WHERE user_id = ?`, userID).Scan(&q.Period, &q.TokenLimit)
	if err == sql.ErrNoRows {
		return q, nil
	}
	return q, err
}

func (s *Store) SetQuota(userID int64, q Quota) error {
	_, err := s.DB.Exec(`INSERT INTO quotas(user_id, period, token_limit) VALUES (?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET period = excluded.period, token_limit = excluded.token_limit`,
		userID, q.Period, q.TokenLimit)
	return err
}

type UsageRow struct {
	Username         string
	ModelName        string
	Requests         int64
	PromptTokens     int64
	CompletionTokens int64
	Cost             float64
	Billable         bool
}

// UsageReport groups usage since a time by user and model (billable separated).
func (s *Store) UsageReport(since time.Time) ([]UsageRow, error) {
	rows, err := s.DB.Query(`SELECT COALESCE(u.username, '(deleted)'), l.model_name, COUNT(*),
		SUM(l.prompt_tokens), SUM(l.completion_tokens), SUM(l.cost), l.billable
		FROM usage_log l LEFT JOIN users u ON u.id = l.user_id
		WHERE l.created_at >= ?
		GROUP BY l.user_id, l.model_name, l.billable
		ORDER BY SUM(l.prompt_tokens + l.completion_tokens) DESC`, FormatTime(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UsageRow
	for rows.Next() {
		var r UsageRow
		if err := rows.Scan(&r.Username, &r.ModelName, &r.Requests, &r.PromptTokens, &r.CompletionTokens, &r.Cost, &r.Billable); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ModelUsageReport groups usage since a time by model only.
func (s *Store) ModelUsageReport(since time.Time) ([]UsageRow, error) {
	rows, err := s.DB.Query(`SELECT model_name, COUNT(*), SUM(prompt_tokens), SUM(completion_tokens), SUM(cost)
		FROM usage_log WHERE created_at >= ? GROUP BY model_name
		ORDER BY SUM(prompt_tokens + completion_tokens) DESC`, FormatTime(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UsageRow
	for rows.Next() {
		var r UsageRow
		if err := rows.Scan(&r.ModelName, &r.Requests, &r.PromptTokens, &r.CompletionTokens, &r.Cost); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
