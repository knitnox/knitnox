package store

import "strconv"

// Settings are admin-tunable values stored in the settings table.
// Add a new one by adding a field, a default, and a line in Load/Save.
type Settings struct {
	AllowSignup            bool
	QueueMax               int     // pending messages per thread
	AgentMaxSteps          int     // LLM <-> tool round trips per turn
	ToolMaxRetries         int     // retries for one failing MCP call
	LLMMaxRetries          int     // retries for 429/5xx/network before first token
	CompactionPairs        int     // compact when un-summarized pairs exceed this
	CompactionKeep         int     // recent pairs kept verbatim after compaction
	CompactionContextRatio float64 // or when estimated tokens exceed ratio * context window
	CompactionModelID      int64   // 0 = use the thread's model
	SystemPrompt           string
	MaxParallelRuns        int // generations running at once across all chats (0 = unlimited)
	ApprovalTimeoutMin     int // minutes a tool call waits for approval before it is denied
	LLMIdleSeconds         int // a model call is given up after this long without data (0 = never)
	RunMaxMinutes          int // a reply or agent run is stopped after this long (0 = never)
	BackupEveryHours       int // automatic backups (0 = off)
	BackupKeep             int // how many automatic backups are kept
	// Long-term memory: the embedding model that powers "smart search" for
	// everyone who has not set their own. Provider 0 or an empty model = off.
	EmbedProviderID int64
	EmbedModel      string
	EmbedDimensions int // 0 = the model's own size
	// Feed: may people share posts with everyone on this server?
	FeedPublic bool
	// Pictures, recordings and videos one person may have made per day with the server's models (0 = no limit).
	FeedMediaDaily int
}

func DefaultSettings() Settings {
	return Settings{
		AllowSignup:            true,
		QueueMax:               10,
		AgentMaxSteps:          10,
		ToolMaxRetries:         2,
		LLMMaxRetries:          2,
		CompactionPairs:        50,
		CompactionKeep:         10,
		CompactionContextRatio: 0.7,
		SystemPrompt:           "You are a helpful assistant. Answer concisely and use Markdown when it helps.",
		MaxParallelRuns:        8,
		ApprovalTimeoutMin:     30,
		LLMIdleSeconds:         300,
		RunMaxMinutes:          60,
		BackupEveryHours:       24,
		BackupKeep:             7,
		FeedPublic:             true,
		FeedMediaDaily:         20,
	}
}

func (s *Store) LoadSettings() (Settings, error) {
	st := DefaultSettings()
	rows, err := s.DB.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return st, err
		}
		atoi := func(def int) int {
			if n, err := strconv.Atoi(v); err == nil {
				return n
			}
			return def
		}
		switch k {
		case "allow_signup":
			st.AllowSignup = v == "1"
		case "queue_max":
			st.QueueMax = atoi(st.QueueMax)
		case "agent_max_steps":
			st.AgentMaxSteps = atoi(st.AgentMaxSteps)
		case "tool_max_retries":
			st.ToolMaxRetries = atoi(st.ToolMaxRetries)
		case "llm_max_retries":
			st.LLMMaxRetries = atoi(st.LLMMaxRetries)
		case "compaction_pairs":
			st.CompactionPairs = atoi(st.CompactionPairs)
		case "compaction_keep":
			st.CompactionKeep = atoi(st.CompactionKeep)
		case "compaction_context_ratio":
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				st.CompactionContextRatio = f
			}
		case "compaction_model_id":
			st.CompactionModelID = int64(atoi(0))
		case "system_prompt":
			st.SystemPrompt = v
		case "max_parallel_runs":
			st.MaxParallelRuns = atoi(st.MaxParallelRuns)
		case "approval_timeout_min":
			st.ApprovalTimeoutMin = atoi(st.ApprovalTimeoutMin)
		case "llm_idle_seconds":
			st.LLMIdleSeconds = atoi(st.LLMIdleSeconds)
		case "run_max_minutes":
			st.RunMaxMinutes = atoi(st.RunMaxMinutes)
		case "backup_every_hours":
			st.BackupEveryHours = atoi(st.BackupEveryHours)
		case "backup_keep":
			st.BackupKeep = atoi(st.BackupKeep)
		case "embed_provider_id":
			st.EmbedProviderID = int64(atoi(0))
		case "embed_model":
			st.EmbedModel = v
		case "feed_public":
			st.FeedPublic = v == "1"
		case "feed_media_daily":
			st.FeedMediaDaily = atoi(st.FeedMediaDaily)
		case "embed_dimensions":
			st.EmbedDimensions = atoi(0)
		}
	}
	return st, rows.Err()
}

func (s *Store) SaveSettings(st Settings) error {
	vals := map[string]string{
		"allow_signup":             strconv.Itoa(b2i(st.AllowSignup)),
		"queue_max":                strconv.Itoa(st.QueueMax),
		"agent_max_steps":          strconv.Itoa(st.AgentMaxSteps),
		"tool_max_retries":         strconv.Itoa(st.ToolMaxRetries),
		"llm_max_retries":          strconv.Itoa(st.LLMMaxRetries),
		"compaction_pairs":         strconv.Itoa(st.CompactionPairs),
		"compaction_keep":          strconv.Itoa(st.CompactionKeep),
		"compaction_context_ratio": strconv.FormatFloat(st.CompactionContextRatio, 'f', 2, 64),
		"compaction_model_id":      strconv.FormatInt(st.CompactionModelID, 10),
		"system_prompt":            st.SystemPrompt,
		"max_parallel_runs":        strconv.Itoa(st.MaxParallelRuns),
		"approval_timeout_min":     strconv.Itoa(st.ApprovalTimeoutMin),
		"llm_idle_seconds":         strconv.Itoa(st.LLMIdleSeconds),
		"run_max_minutes":          strconv.Itoa(st.RunMaxMinutes),
		"backup_every_hours":       strconv.Itoa(st.BackupEveryHours),
		"backup_keep":              strconv.Itoa(st.BackupKeep),
		"embed_provider_id":        strconv.FormatInt(st.EmbedProviderID, 10),
		"embed_model":              st.EmbedModel,
		"embed_dimensions":         strconv.Itoa(st.EmbedDimensions),
		"feed_public":              strconv.Itoa(b2i(st.FeedPublic)),
		"feed_media_daily":         strconv.Itoa(st.FeedMediaDaily),
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for k, v := range vals {
		if _, err := tx.Exec(`INSERT INTO settings(key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value`, k, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}
