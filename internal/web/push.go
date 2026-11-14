package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// ---- VAPID keys ----

// initPush loads or generates the VAPID key pair used to sign push messages.
func (s *Server) initPush() error {
	s.vapidPublic = s.st.GetValue("vapid_public")
	s.vapidPrivate = s.st.GetValue("vapid_private")
	if s.vapidPublic != "" && s.vapidPrivate != "" {
		return nil
	}
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return err
	}
	if err := s.st.SetValue("vapid_public", pub); err != nil {
		return err
	}
	if err := s.st.SetValue("vapid_private", priv); err != nil {
		return err
	}
	s.vapidPublic, s.vapidPrivate = pub, priv
	return nil
}

// ---- who is watching which thread (no push while you are looking at it) ----

func (s *Server) watchThread(threadID, userID int64) func() {
	s.watchMu.Lock()
	if s.watchers[threadID] == nil {
		s.watchers[threadID] = map[int64]int{}
	}
	s.watchers[threadID][userID]++
	s.watchMu.Unlock()
	return func() {
		s.watchMu.Lock()
		defer s.watchMu.Unlock()
		if m := s.watchers[threadID]; m != nil {
			if m[userID]--; m[userID] <= 0 {
				delete(m, userID)
			}
			if len(m) == 0 {
				delete(s.watchers, threadID)
			}
		}
	}
}

func (s *Server) isWatching(threadID, userID int64) bool {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	return s.watchers[threadID][userID] > 0
}

// ---- sending ----

type pushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Tag   string `json:"tag"`
}

func (s *Server) sendPush(userID int64, p pushPayload) {
	subs, err := s.st.PushSubscriptionsFor(userID)
	if err != nil || len(subs) == 0 {
		return
	}
	if s.pushSink != nil {
		s.pushSink(userID, p)
		return
	}
	body, _ := json.Marshal(p)
	for _, sub := range subs {
		resp, err := webpush.SendNotification(body, &webpush.Subscription{
			Endpoint: sub.Endpoint, Keys: webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
		}, &webpush.Options{
			Subscriber: "mailto:admin@localhost", VAPIDPublicKey: s.vapidPublic, VAPIDPrivateKey: s.vapidPrivate,
			TTL: 3600, Urgency: webpush.UrgencyNormal, Topic: p.Tag,
		})
		if err != nil {
			s.log.Warn("push failed", "err", err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
			s.st.DeletePushSubscription(0, sub.Endpoint) // browser unsubscribed
		}
	}
}

func preview(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// ---- subscription endpoints ----

func (s *Server) pushSubscribe(w http.ResponseWriter, r *http.Request) {
	var sub struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&sub); err != nil ||
		!strings.HasPrefix(sub.Endpoint, "https://") || sub.Keys.P256dh == "" || sub.Keys.Auth == "" {
		http.Error(w, "invalid subscription", http.StatusBadRequest)
		return
	}
	if err := s.st.SavePushSubscription(currentUser(r).ID, sub.Endpoint, sub.Keys.P256dh, sub.Keys.Auth); err != nil {
		s.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) pushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	var sub struct {
		Endpoint string `json:"endpoint"`
	}
	json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&sub)
	if sub.Endpoint != "" {
		s.st.DeletePushSubscription(currentUser(r).ID, sub.Endpoint)
	}
	w.WriteHeader(http.StatusNoContent)
}

// pushTest sends a notification to the caller's own devices.
func (s *Server) pushTest(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if s.st.PushSubscriptionCount(u.ID) == 0 {
		http.Error(w, "No devices are subscribed yet.", http.StatusBadRequest)
		return
	}
	s.sendPush(u.ID, pushPayload{Title: "Knitnox", Body: "Notifications are working on this account.", URL: "/", Tag: "test-" + time.Now().Format("150405")})
	w.WriteHeader(http.StatusNoContent)
}
