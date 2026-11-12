package store

import (
	"database/sql"
	"strings"
)

type Provider struct {
	ID          int64
	OwnerUserID sql.NullInt64 // invalid = global
	Name        string
	BaseURL     string
	APIKey      string // decrypted; never render this
	Enabled     bool
	Models      []Model
}

func (p *Provider) IsGlobal() bool { return !p.OwnerUserID.Valid }

type Model struct {
	ID            int64
	ProviderID    int64
	Name          string
	ContextWindow int
	InputPrice    float64
	OutputPrice   float64
	Enabled       bool
	Vision        bool   // accepts image input
	Kind          string // chat | embedding | image | voice | video
}

// ModelOption is a model a user can pick, with its provider.
type ModelOption struct {
	ModelID      int64
	ModelName    string
	ProviderName string
	Personal     bool
	Vision       bool
	Kind         string
}

func NormalizeBaseURL(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimRight(u, "/")
	u = strings.TrimSuffix(u, "/chat/completions")
	return u
}

func (s *Store) CreateProvider(owner *int64, name, baseURL, apiKey string) (int64, error) {
	enc, err := s.Box.Seal(apiKey)
	if err != nil {
		return 0, err
	}
	res, err := s.DB.Exec(`INSERT INTO providers(owner_user_id, name, base_url, api_key_enc) VALUES (?, ?, ?, ?)`,
		owner, name, NormalizeBaseURL(baseURL), enc)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ownerClause matches providers owned by owner (nil = global).
func ownerClause(owner *int64) (string, []any) {
	if owner == nil {
		return "owner_user_id IS NULL", nil
	}
	return "owner_user_id = ?", []any{*owner}
}

func (s *Store) DeleteProvider(owner *int64, id int64) error {
	clause, args := ownerClause(owner)
	_, err := s.DB.Exec(`DELETE FROM providers WHERE id = ? AND `+clause, append([]any{id}, args...)...)
	return err
}

func (s *Store) ToggleProvider(owner *int64, id int64) error {
	clause, args := ownerClause(owner)
	_, err := s.DB.Exec(`UPDATE providers SET enabled = 1 - enabled WHERE id = ? AND `+clause, append([]any{id}, args...)...)
	return err
}

// ProviderOwnedBy reports whether provider id belongs to owner (nil = global).
func (s *Store) ProviderOwnedBy(owner *int64, id int64) bool {
	clause, args := ownerClause(owner)
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM providers WHERE id = ? AND `+clause, append([]any{id}, args...)...).Scan(&n)
	return n > 0
}

// ListProviders returns providers (with models) for owner (nil = global). Keys are decrypted.
func (s *Store) ListProviders(owner *int64) ([]Provider, error) {
	clause, args := ownerClause(owner)
	rows, err := s.DB.Query(`SELECT id, owner_user_id, name, base_url, api_key_enc, enabled
		FROM providers WHERE `+clause+` ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	var out []Provider
	for rows.Next() {
		p, err := s.scanProvider(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *p)
	}
	rows.Close()
	for i := range out {
		if out[i].Models, err = s.ListModels(out[i].ID); err != nil {
			return nil, err
		}
	}
	return out, rows.Err()
}

func (s *Store) scanProvider(sc interface{ Scan(...any) error }) (*Provider, error) {
	p := &Provider{}
	var enc []byte
	if err := sc.Scan(&p.ID, &p.OwnerUserID, &p.Name, &p.BaseURL, &enc, &p.Enabled); err != nil {
		return nil, err
	}
	key, err := s.Box.Open(enc)
	if err != nil {
		return nil, err
	}
	p.APIKey = key
	return p, nil
}

func (s *Store) ProviderByID(id int64) (*Provider, error) {
	p, err := s.scanProvider(s.DB.QueryRow(`SELECT id, owner_user_id, name, base_url, api_key_enc, enabled
		FROM providers WHERE id = ?`, id))
	return p, notFound(err)
}

func (s *Store) ListModels(providerID int64) ([]Model, error) {
	rows, err := s.DB.Query(`SELECT id, provider_id, name, context_window, input_price, output_price, enabled, vision, kind
		FROM models WHERE provider_id = ? ORDER BY kind != 'chat', kind, name`, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Model
	for rows.Next() {
		var m Model
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.Name, &m.ContextWindow, &m.InputPrice, &m.OutputPrice, &m.Enabled, &m.Vision, &m.Kind); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// CreateModel adds a model; what it is for is guessed from its name (see GuessModelKind).
func (s *Store) CreateModel(providerID int64, name string, ctxWindow int, inPrice, outPrice float64, vision bool) error {
	return s.CreateModelKind(providerID, name, GuessModelKind(name), ctxWindow, inPrice, outPrice, vision)
}

// CreateModelKind adds a model of a given kind. An existing model keeps its kind.
func (s *Store) CreateModelKind(providerID int64, name, kind string, ctxWindow int, inPrice, outPrice float64, vision bool) error {
	if !ValidModelKind(kind) {
		kind = ModelChat
	}
	if kind != ModelChat {
		vision = false
	}
	if ctxWindow <= 0 {
		ctxWindow = 128000
	}
	_, err := s.DB.Exec(`INSERT INTO models(provider_id, name, context_window, input_price, output_price, vision, kind)
		VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT(provider_id, name) DO UPDATE SET
		context_window = excluded.context_window, input_price = excluded.input_price,
		output_price = excluded.output_price, vision = excluded.vision`,
		providerID, strings.TrimSpace(name), ctxWindow, inPrice, outPrice, b2i(vision), kind)
	return err
}

func (s *Store) ToggleModelVision(id int64) error {
	_, err := s.DB.Exec(`UPDATE models SET vision = 1 - vision WHERE id = ?`, id)
	return err
}

func (s *Store) ModelByID(id int64) (*Model, error) {
	m := &Model{}
	err := s.DB.QueryRow(`SELECT id, provider_id, name, context_window, input_price, output_price, enabled, vision, kind
		FROM models WHERE id = ?`, id).
		Scan(&m.ID, &m.ProviderID, &m.Name, &m.ContextWindow, &m.InputPrice, &m.OutputPrice, &m.Enabled, &m.Vision, &m.Kind)
	return m, notFound(err)
}

func (s *Store) DeleteModel(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM models WHERE id = ?`, id)
	return err
}

func (s *Store) ToggleModel(id int64) error {
	_, err := s.DB.Exec(`UPDATE models SET enabled = 1 - enabled WHERE id = ?`, id)
	return err
}

// ModelOptions lists enabled models userID may select: global ones plus their own.
func (s *Store) ModelOptions(userID int64) ([]ModelOption, error) {
	rows, err := s.DB.Query(`SELECT m.id, m.name, p.name, p.owner_user_id IS NOT NULL, m.vision
		FROM models m JOIN providers p ON p.id = m.provider_id
		WHERE m.enabled = 1 AND p.enabled = 1 AND m.kind = 'chat' AND (p.owner_user_id IS NULL OR p.owner_user_id = ?)
		ORDER BY p.owner_user_id IS NOT NULL, p.name, m.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModelOption
	for rows.Next() {
		var o ModelOption
		if err := rows.Scan(&o.ModelID, &o.ModelName, &o.ProviderName, &o.Personal, &o.Vision); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// AllGlobalModelOptions is used by the admin settings page (compaction model).
func (s *Store) AllGlobalModelOptions() ([]ModelOption, error) {
	rows, err := s.DB.Query(`SELECT m.id, m.name, p.name FROM models m JOIN providers p ON p.id = m.provider_id
		WHERE p.owner_user_id IS NULL AND m.kind = 'chat' ORDER BY p.name, m.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModelOption
	for rows.Next() {
		var o ModelOption
		if err := rows.Scan(&o.ModelID, &o.ModelName, &o.ProviderName); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// What a model is for.
const (
	ModelChat      = "chat"
	ModelEmbedding = "embedding"
	ModelImage     = "image"
	ModelVoice     = "voice"
	ModelVideo     = "video"
)

func ValidModelKind(k string) bool {
	switch k {
	case ModelChat, ModelEmbedding, ModelImage, ModelVoice, ModelVideo:
		return true
	}
	return false
}

// GuessModelKind guesses from a model's name what it is for. It is only a
// first guess when models are added in bulk; the kind can be changed afterwards.
func GuessModelKind(name string) string {
	n := strings.ToLower(name)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(n, w) {
				return true
			}
		}
		return false
	}
	switch {
	case has("embed", "bge-", "e5-", "gte-"):
		return ModelEmbedding
	case has("tts", "speech", "voice", "kokoro", "elevenlabs"):
		return ModelVoice
	case has("sora", "video", "veo", "kling", "wan-", "wan2"):
		return ModelVideo
	case has("dall-e", "dalle", "gpt-image", "image", "flux", "stable-diffusion", "sdxl", "imagen", "ideogram", "recraft"):
		return ModelImage
	}
	return ModelChat
}

// SetModelKind changes what a model is for.
func (s *Store) SetModelKind(id int64, kind string) error {
	if !ValidModelKind(kind) {
		return ErrNotFound
	}
	_, err := s.DB.Exec(`UPDATE models SET kind = ?, vision = CASE WHEN ? = 'chat' THEN vision ELSE 0 END WHERE id = ?`, kind, kind, id)
	return err
}

// KindModelOptions lists the enabled models of one kind that userID may use:
// the server's shared ones first, then their own.
func (s *Store) KindModelOptions(userID int64, kind string) ([]ModelOption, error) {
	rows, err := s.DB.Query(`SELECT m.id, m.name, p.name, p.owner_user_id IS NOT NULL, m.vision, m.kind
		FROM models m JOIN providers p ON p.id = m.provider_id
		WHERE m.enabled = 1 AND p.enabled = 1 AND m.kind = ? AND (p.owner_user_id IS NULL OR p.owner_user_id = ?)
		ORDER BY p.owner_user_id IS NOT NULL, p.id, m.name`, kind, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModelOption
	for rows.Next() {
		var o ModelOption
		if err := rows.Scan(&o.ModelID, &o.ModelName, &o.ProviderName, &o.Personal, &o.Vision, &o.Kind); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ModelIDByName finds a provider's model by its name (0 = none).
func (s *Store) ModelIDByName(providerID int64, name string) int64 {
	var id int64
	s.DB.QueryRow(`SELECT id FROM models WHERE provider_id = ? AND name = ?`, providerID, strings.TrimSpace(name)).Scan(&id)
	return id
}
