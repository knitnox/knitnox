package store

import (
	"database/sql"
	"encoding/binary"
	"errors"
	"math"
	"strings"
)

// Long-term memory. A memory is one short fact. It can be "about" things
// (entities: people, places, projects) and can state connections between
// them (relations: Sam -works at-> Acme Bakery). Every row belongs to one
// user and every query here filters by that user.

// Limits applied wherever a memory is created or edited.
const (
	MemoryMaxLen       = 2000 // characters of one memory
	MemoryMaxAbout     = 12   // things one memory can be about
	MemoryMaxRelations = 12   // connections one memory can state
	memoryNameMax      = 80
	memoryPredicateMax = 60
)

// Who saved a memory.
const (
	MemoryFromUser = "user"
	MemoryFromAI   = "ai"
)

// ErrEmptyMemory is returned when a memory has no text.
var ErrEmptyMemory = errors.New("a memory needs some text")

// MemorySettings is one user's memory switch and optional own embedding model.
type MemorySettings struct {
	Enabled         bool   // the AI may read and write this user's memory
	EmbedProviderID int64  // 0 = use the server's embedding model
	EmbedModel      string // model id at that provider
	EmbedDimensions int    // 0 = the model's own size
}

func (s *Store) MemorySettingsFor(userID int64) (MemorySettings, error) {
	var ms MemorySettings
	err := s.DB.QueryRow(`SELECT enabled, COALESCE(embed_provider_id, 0), embed_model, embed_dimensions
		FROM memory_settings WHERE user_id = ?`, userID).
		Scan(&ms.Enabled, &ms.EmbedProviderID, &ms.EmbedModel, &ms.EmbedDimensions)
	if errors.Is(err, sql.ErrNoRows) {
		return ms, nil
	}
	return ms, err
}

func (s *Store) SaveMemorySettings(userID int64, ms MemorySettings) error {
	var pid any
	if ms.EmbedProviderID > 0 {
		pid = ms.EmbedProviderID
	}
	_, err := s.DB.Exec(`INSERT INTO memory_settings(user_id, enabled, embed_provider_id, embed_model, embed_dimensions)
		VALUES (?, ?, ?, ?, ?) ON CONFLICT(user_id) DO UPDATE SET enabled = excluded.enabled,
		embed_provider_id = excluded.embed_provider_id, embed_model = excluded.embed_model,
		embed_dimensions = excluded.embed_dimensions`,
		userID, b2i(ms.Enabled), pid, strings.TrimSpace(ms.EmbedModel), max(ms.EmbedDimensions, 0))
	return err
}

// MemoryEnabled reports whether the AI may use this user's memory.
func (s *Store) MemoryEnabled(userID int64) bool {
	ms, err := s.MemorySettingsFor(userID)
	return err == nil && ms.Enabled
}

// Memory is one remembered fact with the things it is about.
type Memory struct {
	ID             int64
	UserID         int64
	Content        string
	Source         string // user | ai
	SourceThreadID int64  // chat it was saved in (0 = none, or the chat is gone)
	SourceTitle    string
	Pinned         bool
	Indexed        bool // has a vector for smart search
	CreatedAt      string
	UpdatedAt      string
	Entities       []MemoryEntity
	Relations      []MemoryRelation
	Snippet        string // search results only: plain text with [[ ]] around matches
}

// MemoryEntity is a thing memories talk about.
type MemoryEntity struct {
	ID       int64
	Name     string
	Memories int // memories about it
	Links    int // connections to other things
}

// MemoryRelation is one connection: Subject -Predicate-> Object.
type MemoryRelation struct {
	ID        int64
	MemoryID  int64
	SubjectID int64
	Subject   string
	Predicate string
	ObjectID  int64
	Object    string
}

// RelationInput is a connection as typed by a person or sent by the AI.
type RelationInput struct {
	Subject   string `json:"subject"`
	Predicate string `json:"relation"`
	Object    string `json:"object"`
}

// MemoryInput is what is needed to create or replace a memory.
type MemoryInput struct {
	Content   string
	About     []string
	Relations []RelationInput
	Source    string // MemoryFromUser (default) or MemoryFromAI; ignored on update
	ThreadID  int64  // chat it was saved in; ignored on update
	Pinned    bool   // ignored on update
}

func cleanName(s string) string {
	return clip(strings.Join(strings.Fields(s), " "), memoryNameMax)
}

// cleanPredicate turns "WORKS_AT" and " works  at " into "works at".
func cleanPredicate(s string) string {
	s = strings.ToLower(strings.ReplaceAll(s, "_", " "))
	return clip(strings.Join(strings.Fields(s), " "), memoryPredicateMax)
}

func (in *MemoryInput) normalize() {
	in.Content = clip(strings.TrimSpace(in.Content), MemoryMaxLen)
	seen := map[string]bool{}
	var about []string
	for _, a := range in.About {
		a = cleanName(a)
		if k := strings.ToLower(a); a != "" && !seen[k] && len(about) < MemoryMaxAbout {
			seen[k] = true
			about = append(about, a)
		}
	}
	in.About = about
	seenRel := map[string]bool{}
	var rels []RelationInput
	for _, r := range in.Relations {
		r = RelationInput{cleanName(r.Subject), cleanPredicate(r.Predicate), cleanName(r.Object)}
		if r.Subject == "" || r.Predicate == "" || r.Object == "" || strings.EqualFold(r.Subject, r.Object) {
			continue
		}
		k := strings.ToLower(r.Subject + "\x00" + r.Predicate + "\x00" + r.Object)
		if !seenRel[k] && len(rels) < MemoryMaxRelations {
			seenRel[k] = true
			rels = append(rels, r)
		}
	}
	in.Relations = rels
}

// entityID returns the id of the user's entity with this name, creating it if needed.
func entityID(tx *sql.Tx, userID int64, name string) (int64, error) {
	key := strings.ToLower(name)
	if _, err := tx.Exec(`INSERT INTO memory_entities(user_id, name, name_key) VALUES (?, ?, ?)
		ON CONFLICT(user_id, name_key) DO NOTHING`, userID, name, key); err != nil {
		return 0, err
	}
	var id int64
	err := tx.QueryRow(`SELECT id FROM memory_entities WHERE user_id = ? AND name_key = ?`, userID, key).Scan(&id)
	return id, err
}

// linkMemory replaces what a memory is about and the connections it states,
// refreshes its search row and drops things no memory mentions any more.
func linkMemory(tx *sql.Tx, userID, memoryID int64, in MemoryInput) error {
	if _, err := tx.Exec(`DELETE FROM memory_relations WHERE memory_id = ?`, memoryID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM memory_mentions WHERE memory_id = ?`, memoryID); err != nil {
		return err
	}
	ids := map[string]int64{}
	resolve := func(name string) (int64, error) {
		k := strings.ToLower(name)
		if id, ok := ids[k]; ok {
			return id, nil
		}
		id, err := entityID(tx, userID, name)
		if err != nil {
			return 0, err
		}
		ids[k] = id
		if _, err := tx.Exec(`INSERT OR IGNORE INTO memory_mentions(memory_id, entity_id) VALUES (?, ?)`, memoryID, id); err != nil {
			return 0, err
		}
		return id, nil
	}
	for _, name := range in.About {
		if _, err := resolve(name); err != nil {
			return err
		}
	}
	for _, r := range in.Relations {
		sid, err := resolve(r.Subject)
		if err != nil {
			return err
		}
		oid, err := resolve(r.Object)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO memory_relations(user_id, memory_id, subject_id, predicate, object_id)
			VALUES (?, ?, ?, ?, ?)`, userID, memoryID, sid, r.Predicate, oid); err != nil {
			return err
		}
	}
	if err := refreshMemoryFTS(tx, memoryID); err != nil {
		return err
	}
	return dropOrphanEntities(tx, userID)
}

func dropOrphanEntities(tx *sql.Tx, userID int64) error {
	_, err := tx.Exec(`DELETE FROM memory_entities WHERE user_id = ?
		AND id NOT IN (SELECT entity_id FROM memory_mentions)`, userID)
	return err
}

// refreshMemoryFTS rewrites the word-search row of one memory.
func refreshMemoryFTS(tx *sql.Tx, memoryID int64) error {
	var content, about string
	if err := tx.QueryRow(`SELECT m.content, COALESCE((SELECT group_concat(e.name, ' · ') FROM memory_mentions mm
		JOIN memory_entities e ON e.id = mm.entity_id WHERE mm.memory_id = m.id), '')
		FROM memories m WHERE m.id = ?`, memoryID).Scan(&content, &about); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM memories_fts WHERE rowid = ?`, memoryID); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO memories_fts(rowid, content, about) VALUES (?, ?, ?)`, memoryID, content, about)
	return err
}

// CreateMemory saves a new memory for userID.
func (s *Store) CreateMemory(userID int64, in MemoryInput) (int64, error) {
	in.normalize()
	if in.Content == "" {
		return 0, ErrEmptyMemory
	}
	source := MemoryFromUser
	if in.Source == MemoryFromAI {
		source = MemoryFromAI
	}
	var thread any
	if in.ThreadID > 0 {
		thread = in.ThreadID
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO memories(user_id, content, source, source_thread_id, pinned) VALUES (?, ?, ?, ?, ?)`,
		userID, in.Content, source, thread, b2i(in.Pinned))
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if err := linkMemory(tx, userID, id, in); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// UpdateMemory replaces the text, the things and the connections of a memory.
// When the text changes its vector is dropped, so it gets indexed again.
func (s *Store) UpdateMemory(userID, id int64, in MemoryInput) error {
	in.normalize()
	if in.Content == "" {
		return ErrEmptyMemory
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old string
	if err := tx.QueryRow(`SELECT content FROM memories WHERE id = ? AND user_id = ?`, id, userID).Scan(&old); err != nil {
		return notFound(err)
	}
	if old != in.Content {
		if _, err := tx.Exec(`UPDATE memories SET content = ?, embedding = NULL, embed_key = '', updated_at = ? WHERE id = ?`,
			in.Content, Now(), id); err != nil {
			return err
		}
	} else if _, err := tx.Exec(`UPDATE memories SET updated_at = ? WHERE id = ?`, Now(), id); err != nil {
		return err
	}
	if err := linkMemory(tx, userID, id, in); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetMemoryPinned(userID, id int64, pinned bool) error {
	res, err := s.DB.Exec(`UPDATE memories SET pinned = ? WHERE id = ? AND user_id = ?`, b2i(pinned), id, userID)
	return affected(res, err)
}

func (s *Store) DeleteMemory(userID, id int64) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM memories WHERE id = ? AND user_id = ?`, id, userID)
	if err := affected(res, err); err != nil {
		return err
	}
	if err := dropOrphanEntities(tx, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteAllMemories wipes a user's whole memory.
func (s *Store) DeleteAllMemories(userID int64) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM memories WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM memory_entities WHERE user_id = ?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// affected turns "no row changed" into ErrNotFound.
func affected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

const memoryCols = `m.id, m.user_id, m.content, m.source, COALESCE(m.source_thread_id, 0), COALESCE(t.title, ''),
	m.pinned, m.embedding IS NOT NULL, m.created_at, m.updated_at`
const memoryFrom = ` FROM memories m LEFT JOIN threads t ON t.id = m.source_thread_id `

func scanMemories(rows *sql.Rows) ([]Memory, error) {
	defer rows.Close()
	var out []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.UserID, &m.Content, &m.Source, &m.SourceThreadID, &m.SourceTitle,
			&m.Pinned, &m.Indexed, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// placeholders returns "?, ?, ?" and the ids as arguments.
func placeholders(ids []int64) (string, []any) {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", "), args
}

// attachLinks fills Entities and Relations of each memory with two queries.
func (s *Store) attachLinks(mems []Memory) error {
	if len(mems) == 0 {
		return nil
	}
	at := map[int64]int{}
	ids := make([]int64, len(mems))
	for i, m := range mems {
		at[m.ID], ids[i] = i, m.ID
	}
	in, args := placeholders(ids)
	rows, err := s.DB.Query(`SELECT mm.memory_id, e.id, e.name FROM memory_mentions mm
		JOIN memory_entities e ON e.id = mm.entity_id WHERE mm.memory_id IN (`+in+`) ORDER BY e.name`, args...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var mid int64
		var e MemoryEntity
		if err := rows.Scan(&mid, &e.ID, &e.Name); err != nil {
			rows.Close()
			return err
		}
		mems[at[mid]].Entities = append(mems[at[mid]].Entities, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	rels, err := s.queryRelations(`r.memory_id IN (`+in+`) ORDER BY r.id`, args...)
	if err != nil {
		return err
	}
	for _, r := range rels {
		mems[at[r.MemoryID]].Relations = append(mems[at[r.MemoryID]].Relations, r)
	}
	return nil
}

func (s *Store) queryRelations(where string, args ...any) ([]MemoryRelation, error) {
	rows, err := s.DB.Query(`SELECT r.id, r.memory_id, r.subject_id, su.name, r.predicate, r.object_id, ob.name
		FROM memory_relations r JOIN memory_entities su ON su.id = r.subject_id
		JOIN memory_entities ob ON ob.id = r.object_id WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemoryRelation
	for rows.Next() {
		var r MemoryRelation
		if err := rows.Scan(&r.ID, &r.MemoryID, &r.SubjectID, &r.Subject, &r.Predicate, &r.ObjectID, &r.Object); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) listMemories(where string, args ...any) ([]Memory, error) {
	rows, err := s.DB.Query(`SELECT `+memoryCols+memoryFrom+`WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	mems, err := scanMemories(rows)
	if err != nil {
		return nil, err
	}
	return mems, s.attachLinks(mems)
}

// MemoryByID returns one of the user's memories with its links.
func (s *Store) MemoryByID(userID, id int64) (*Memory, error) {
	mems, err := s.listMemories(`m.id = ? AND m.user_id = ?`, id, userID)
	if err != nil {
		return nil, err
	}
	if len(mems) == 0 {
		return nil, ErrNotFound
	}
	return &mems[0], nil
}

// ListMemories returns a page of memories, newest first. filter is "" (all),
// "pinned", "ai" or "user"; before > 0 continues after that id.
func (s *Store) ListMemories(userID int64, filter string, before int64, limit int) ([]Memory, error) {
	where, args := `m.user_id = ?`, []any{userID}
	switch filter {
	case "pinned":
		where += ` AND m.pinned = 1`
	case MemoryFromAI, MemoryFromUser:
		where += ` AND m.source = ?`
		args = append(args, filter)
	}
	if before > 0 {
		where += ` AND m.id < ?`
		args = append(args, before)
	}
	return s.listMemories(where+` ORDER BY m.id DESC LIMIT ?`, append(args, limit)...)
}

// PinnedMemories are the memories the AI is always shown.
func (s *Store) PinnedMemories(userID int64, limit int) ([]Memory, error) {
	rows, err := s.DB.Query(`SELECT `+memoryCols+memoryFrom+`WHERE m.user_id = ? AND m.pinned = 1 ORDER BY m.id LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	return scanMemories(rows)
}

// MemoriesByIDs returns the user's memories with these ids, in the given order.
func (s *Store) MemoriesByIDs(userID int64, ids []int64) ([]Memory, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	in, args := placeholders(ids)
	mems, err := s.listMemories(`m.user_id = ? AND m.id IN (`+in+`)`, append([]any{userID}, args...)...)
	if err != nil {
		return nil, err
	}
	byID := map[int64]Memory{}
	for _, m := range mems {
		byID[m.ID] = m
	}
	out := make([]Memory, 0, len(ids))
	for _, id := range ids {
		if m, ok := byID[id]; ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// MemoryIDByContent returns the id of a memory with exactly this text
// (ignoring letter case), or 0. It stops the AI saving the same fact twice.
func (s *Store) MemoryIDByContent(userID int64, content string) int64 {
	var id int64
	s.DB.QueryRow(`SELECT id FROM memories WHERE user_id = ? AND lower(content) = lower(?) LIMIT 1`,
		userID, clip(strings.TrimSpace(content), MemoryMaxLen)).Scan(&id)
	return id
}

// MemoryStats are the numbers shown at the top of the Memory page.
type MemoryStats struct {
	Memories, Pinned, Entities, Relations int
	Indexed                               int // memories with a vector from the current embedding model
}

func (s *Store) MemoryStats(userID int64, embedKey string) MemoryStats {
	var st MemoryStats
	s.DB.QueryRow(`SELECT COUNT(*), COALESCE(SUM(pinned), 0),
		COALESCE(SUM(embedding IS NOT NULL AND embed_key = ? AND ? != ''), 0)
		FROM memories WHERE user_id = ?`, embedKey, embedKey, userID).Scan(&st.Memories, &st.Pinned, &st.Indexed)
	s.DB.QueryRow(`SELECT COUNT(*) FROM memory_entities WHERE user_id = ?`, userID).Scan(&st.Entities)
	s.DB.QueryRow(`SELECT COUNT(*) FROM memory_relations WHERE user_id = ?`, userID).Scan(&st.Relations)
	return st
}

// ---- word search ----

// Words too common to help find a memory.
var memoryStopWords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`a an and are as at be by did do does for from had has have how i in is it me my of on or
		that the their they this to was we were what when where which who whom whose why will with you your about`) {
		memoryStopWords[w] = true
	}
}

// ftsAny turns free text into an FTS5 query matching any of its words (as
// prefixes). Ranking then puts memories with more of the words first.
func ftsAny(q string) string {
	var terms, all []string
	for _, w := range strings.Fields(q) {
		w = strings.Trim(strings.ReplaceAll(w, `"`, ""), `.,;:!?()[]{}'`)
		if w == "" {
			continue
		}
		term := `"` + w + `"*`
		all = append(all, term)
		if !memoryStopWords[strings.ToLower(w)] {
			terms = append(terms, term)
		}
	}
	if len(terms) == 0 {
		terms = all
	}
	return strings.Join(terms, " OR ")
}

// MemoryHit is one word-search result, best first.
type MemoryHit struct {
	ID      int64
	Snippet string // plain text with [[ ]] around matches
}

// SearchMemoryText finds the user's memories containing any word of q, in the
// text or in the names of the things it is about.
func (s *Store) SearchMemoryText(userID int64, q string, limit int) ([]MemoryHit, error) {
	fq := ftsAny(q)
	if fq == "" {
		return nil, nil
	}
	rows, err := s.DB.Query(`SELECT m.id, snippet(memories_fts, 0, '[[', ']]', '…', 28)
		FROM memories_fts JOIN memories m ON m.id = memories_fts.rowid
		WHERE memories_fts MATCH ? AND m.user_id = ?
		ORDER BY bm25(memories_fts), m.id DESC LIMIT ?`, fq, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemoryHit
	for rows.Next() {
		var h MemoryHit
		if err := rows.Scan(&h.ID, &h.Snippet); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// ---- vectors (smart search) ----

// MemoryVector is one memory's embedding.
type MemoryVector struct {
	ID  int64
	Vec []float32
}

// EncodeVector stores float32s as little-endian bytes.
func EncodeVector(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(f))
	}
	return b
}

func DecodeVector(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return v
}

// MemoryVectors loads every vector of the user made by the model embedKey.
func (s *Store) MemoryVectors(userID int64, embedKey string) ([]MemoryVector, error) {
	rows, err := s.DB.Query(`SELECT id, embedding FROM memories
		WHERE user_id = ? AND embed_key = ? AND embedding IS NOT NULL`, userID, embedKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemoryVector
	for rows.Next() {
		var id int64
		var b []byte
		if err := rows.Scan(&id, &b); err != nil {
			return nil, err
		}
		out = append(out, MemoryVector{id, DecodeVector(b)})
	}
	return out, rows.Err()
}

// MemoriesToEmbed returns memories after afterID that have no vector from embedKey yet.
func (s *Store) MemoriesToEmbed(userID int64, embedKey string, afterID int64, limit int) ([]Memory, error) {
	rows, err := s.DB.Query(`SELECT id, content FROM memories
		WHERE user_id = ? AND id > ? AND (embedding IS NULL OR embed_key != ?) ORDER BY id LIMIT ?`, userID, afterID, embedKey, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Content); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetMemoryEmbedding stores a vector, unless the text changed while it was made.
func (s *Store) SetMemoryEmbedding(userID, id int64, content, embedKey string, vec []float32) error {
	_, err := s.DB.Exec(`UPDATE memories SET embedding = ?, embed_key = ? WHERE id = ? AND user_id = ? AND content = ?`,
		EncodeVector(vec), embedKey, id, userID, content)
	return err
}

// ---- things and connections ----

const entityCols = `e.id, e.name,
	(SELECT COUNT(*) FROM memory_mentions mm WHERE mm.entity_id = e.id),
	(SELECT COUNT(*) FROM memory_relations r WHERE r.subject_id = e.id OR r.object_id = e.id)`

func scanEntities(rows *sql.Rows) ([]MemoryEntity, error) {
	defer rows.Close()
	var out []MemoryEntity
	for rows.Next() {
		var e MemoryEntity
		if err := rows.Scan(&e.ID, &e.Name, &e.Memories, &e.Links); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// ListEntities returns the user's things, the most connected first. q filters by name.
func (s *Store) ListEntities(userID int64, q string, limit int) ([]MemoryEntity, error) {
	where, args := `e.user_id = ?`, []any{userID}
	if q = strings.ToLower(strings.TrimSpace(q)); q != "" {
		where += ` AND e.name_key LIKE ? ESCAPE '\'`
		args = append(args, "%"+likeEscaper.Replace(q)+"%")
	}
	rows, err := s.DB.Query(`SELECT `+entityCols+` FROM memory_entities e WHERE `+where+`
		ORDER BY 4 DESC, 3 DESC, e.name LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	return scanEntities(rows)
}

func (s *Store) EntityByID(userID, id int64) (*MemoryEntity, error) {
	rows, err := s.DB.Query(`SELECT `+entityCols+` FROM memory_entities e WHERE e.id = ? AND e.user_id = ?`, id, userID)
	if err != nil {
		return nil, err
	}
	list, err := scanEntities(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return &list[0], nil
}

// EntityByName finds a thing by its exact name (any letter case) or, failing
// that, the best-known thing whose name contains the text.
func (s *Store) EntityByName(userID int64, name string) (*MemoryEntity, error) {
	name = cleanName(name)
	if name == "" {
		return nil, ErrNotFound
	}
	var id int64
	err := s.DB.QueryRow(`SELECT id FROM memory_entities WHERE user_id = ? AND name_key = ?`, userID, strings.ToLower(name)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		list, lerr := s.ListEntities(userID, name, 1)
		if lerr != nil {
			return nil, lerr
		}
		if len(list) == 0 {
			return nil, ErrNotFound
		}
		return &list[0], nil
	}
	if err != nil {
		return nil, err
	}
	return s.EntityByID(userID, id)
}

// EntityRelations returns a page of a thing's connections and how many it has.
func (s *Store) EntityRelations(userID, entityID int64, limit, offset int) ([]MemoryRelation, int, error) {
	var total int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM memory_relations WHERE user_id = ? AND (subject_id = ? OR object_id = ?)`,
		userID, entityID, entityID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rels, err := s.queryRelations(`r.user_id = ? AND (r.subject_id = ? OR r.object_id = ?) ORDER BY r.id DESC LIMIT ? OFFSET ?`,
		userID, entityID, entityID, limit, max(offset, 0))
	return rels, total, err
}

// EntityMemories returns the newest memories about a thing and how many there are.
func (s *Store) EntityMemories(userID, entityID int64, limit int) ([]Memory, int, error) {
	var total int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM memory_mentions mm JOIN memories m ON m.id = mm.memory_id
		WHERE mm.entity_id = ? AND m.user_id = ?`, entityID, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	mems, err := s.listMemories(`m.user_id = ? AND m.id IN (SELECT memory_id FROM memory_mentions WHERE entity_id = ?)
		ORDER BY m.id DESC LIMIT ?`, userID, entityID, limit)
	return mems, total, err
}

// RenameEntity renames a thing. If another thing already has the new name the
// two are merged, which is how duplicates ("Sam", "Sam B.") are cleaned up.
// It returns the id that holds the name afterwards.
func (s *Store) RenameEntity(userID, id int64, name string) (int64, error) {
	name = cleanName(name)
	if name == "" {
		return 0, errors.New("a name is required")
	}
	key := strings.ToLower(name)
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM memory_entities WHERE id = ? AND user_id = ?`, id, userID).Scan(&n); err != nil || n == 0 {
		return 0, ErrNotFound
	}
	var other int64
	err = tx.QueryRow(`SELECT id FROM memory_entities WHERE user_id = ? AND name_key = ? AND id != ?`, userID, key, id).Scan(&other)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	keep := id
	if other > 0 { // merge id into other
		keep = other
		steps := []struct {
			q    string
			args []any
		}{
			{`INSERT OR IGNORE INTO memory_mentions(memory_id, entity_id) SELECT memory_id, ? FROM memory_mentions WHERE entity_id = ?`, []any{other, id}},
			{`UPDATE memory_relations SET subject_id = ? WHERE subject_id = ?`, []any{other, id}},
			{`UPDATE memory_relations SET object_id = ? WHERE object_id = ?`, []any{other, id}},
			{`DELETE FROM memory_relations WHERE subject_id = object_id AND subject_id = ?`, []any{other}},
			{`DELETE FROM memory_entities WHERE id = ?`, []any{id}}, // its remaining mentions go with it
		}
		for _, st := range steps {
			if _, err := tx.Exec(st.q, st.args...); err != nil {
				return 0, err
			}
		}
	} else if _, err := tx.Exec(`UPDATE memory_entities SET name = ?, name_key = ? WHERE id = ?`, name, key, id); err != nil {
		return 0, err
	}
	// The names are part of the word search; refresh the memories about it.
	rows, err := tx.Query(`SELECT memory_id FROM memory_mentions WHERE entity_id = ?`, keep)
	if err != nil {
		return 0, err
	}
	var mids []int64
	for rows.Next() {
		var mid int64
		if err := rows.Scan(&mid); err != nil {
			rows.Close()
			return 0, err
		}
		mids = append(mids, mid)
	}
	rows.Close()
	for _, mid := range mids {
		if err := refreshMemoryFTS(tx, mid); err != nil {
			return 0, err
		}
	}
	return keep, tx.Commit()
}

// MemoryGraph is a small part of the user's things and connections, sized for
// a screen (or for an AI's context), never the whole graph.
type MemoryGraph struct {
	Nodes []MemoryEntity
	Edges []MemoryRelation
	More  bool // there is more than was returned
}

// MemoryGraphAround returns the picture drawn on the Memory page. With
// focus == 0 it is the most connected things and the links between them;
// otherwise it is one thing and its direct neighbours. limit caps the nodes.
func (s *Store) MemoryGraphAround(userID, focus int64, limit int) (*MemoryGraph, error) {
	g := &MemoryGraph{}
	if focus == 0 {
		nodes, err := s.ListEntities(userID, "", limit+1)
		if err != nil {
			return nil, err
		}
		if len(nodes) > limit {
			nodes, g.More = nodes[:limit], true
		}
		g.Nodes = nodes
		if len(nodes) == 0 {
			return g, nil
		}
		ids := make([]int64, len(nodes))
		for i, n := range nodes {
			ids[i] = n.ID
		}
		in, args := placeholders(ids)
		all := append(append([]any{userID}, args...), args...)
		g.Edges, err = s.queryRelations(`r.user_id = ? AND r.subject_id IN (`+in+`) AND r.object_id IN (`+in+`)
			GROUP BY r.subject_id, r.predicate, r.object_id ORDER BY r.id DESC LIMIT 400`, all...)
		return g, err
	}
	center, err := s.EntityByID(userID, focus)
	if err != nil {
		return nil, err
	}
	edges, err := s.queryRelations(`r.user_id = ? AND (r.subject_id = ? OR r.object_id = ?)
		GROUP BY r.subject_id, r.predicate, r.object_id ORDER BY r.id DESC LIMIT ?`, userID, focus, focus, limit+1)
	if err != nil {
		return nil, err
	}
	if len(edges) > limit {
		edges, g.More = edges[:limit], true
	}
	g.Edges = edges
	g.Nodes = []MemoryEntity{*center}
	seen := map[int64]bool{focus: true}
	var ids []int64
	for _, e := range edges {
		for _, id := range []int64{e.SubjectID, e.ObjectID} {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	if len(ids) > 0 {
		in, args := placeholders(ids)
		rows, err := s.DB.Query(`SELECT `+entityCols+` FROM memory_entities e WHERE e.user_id = ? AND e.id IN (`+in+`)`,
			append([]any{userID}, args...)...)
		if err != nil {
			return nil, err
		}
		more, err := scanEntities(rows)
		if err != nil {
			return nil, err
		}
		g.Nodes = append(g.Nodes, more...)
	}
	return g, nil
}

// SetAgentUseMemory lets an agent read (never write) its owner's memory.
func (s *Store) SetAgentUseMemory(agentID int64, on bool) error {
	_, err := s.DB.Exec(`UPDATE agents SET use_memory = ? WHERE id = ?`, b2i(on), agentID)
	return err
}
