package store

// PushSubscription is a browser's Web Push endpoint.
type PushSubscription struct {
	ID       int64
	UserID   int64
	Endpoint string
	P256dh   string
	Auth     string
}

func (s *Store) SavePushSubscription(userID int64, endpoint, p256dh, auth string) error {
	_, err := s.DB.Exec(`INSERT INTO push_subscriptions(user_id, endpoint, p256dh, auth) VALUES (?, ?, ?, ?)
		ON CONFLICT(endpoint) DO UPDATE SET user_id = excluded.user_id, p256dh = excluded.p256dh, auth = excluded.auth`,
		userID, endpoint, p256dh, auth)
	return err
}

// DeletePushSubscription removes one endpoint (userID 0 = any owner, used for
// endpoints the push service reports as gone).
func (s *Store) DeletePushSubscription(userID int64, endpoint string) error {
	if userID == 0 {
		_, err := s.DB.Exec(`DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint)
		return err
	}
	_, err := s.DB.Exec(`DELETE FROM push_subscriptions WHERE endpoint = ? AND user_id = ?`, endpoint, userID)
	return err
}

func (s *Store) PushSubscriptionsFor(userID int64) ([]PushSubscription, error) {
	rows, err := s.DB.Query(`SELECT id, user_id, endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PushSubscription
	for rows.Next() {
		var p PushSubscription
		if err := rows.Scan(&p.ID, &p.UserID, &p.Endpoint, &p.P256dh, &p.Auth); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PushUserIDs lists the users who have at least one device that takes notifications.
func (s *Store) PushUserIDs() []int64 {
	rows, err := s.DB.Query(`SELECT DISTINCT user_id FROM push_subscriptions ORDER BY user_id`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			out = append(out, id)
		}
	}
	return out
}

func (s *Store) PushSubscriptionCount(userID int64) int {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM push_subscriptions WHERE user_id = ?`, userID).Scan(&n)
	return n
}

// GetValue / SetValue read and write arbitrary keys in the settings table
// (used for generated secrets such as VAPID keys).
func (s *Store) GetValue(key string) string {
	var v string
	s.DB.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	return v
}

func (s *Store) SetValue(key, value string) error {
	_, err := s.DB.Exec(`INSERT INTO settings(key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
