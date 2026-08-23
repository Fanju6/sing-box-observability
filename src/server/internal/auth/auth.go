package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	CookieName     = "sbox_observability_session"
	maxSessions    = 4096
	maxAttemptKeys = 4096
)

type Manager struct {
	token          string
	ttl            time.Duration
	secure         bool
	mu             sync.Mutex
	sessions       map[string]time.Time
	attempts       map[string][]time.Time
	global         []time.Time
	trustedProxies []string
}

func New(token string, ttl time.Duration, secure bool) *Manager {
	return &Manager{token: token, ttl: ttl, secure: secure, sessions: make(map[string]time.Time), attempts: make(map[string][]time.Time)}
}
func (m *Manager) Enabled() bool { return m.token != "" }

func (m *Manager) SetTrustedProxies(proxies []string) {
	m.mu.Lock()
	m.trustedProxies = append([]string(nil), proxies...)
	m.mu.Unlock()
}

func (m *Manager) Authenticated(r *http.Request) bool {
	if !m.Enabled() {
		return true
	}
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return false
	}
	key := hash(cookie.Value)
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneSessionsLocked(now)
	expiry, ok := m.sessions[key]
	if !ok {
		return false
	}
	return now.Before(expiry)
}

func (m *Manager) Login(w http.ResponseWriter, r *http.Request, supplied string) (bool, time.Duration) {
	if !m.Enabled() {
		return true, 0
	}
	m.mu.Lock()
	trusted := append([]string(nil), m.trustedProxies...)
	m.mu.Unlock()
	ip := remoteIP(r, trusted)
	if !m.allow(ip) {
		return false, time.Minute
	}
	if subtle.ConstantTimeCompare([]byte(supplied), []byte(m.token)) != 1 {
		return false, 0
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return false, 0
	}
	value := hex.EncodeToString(raw)
	m.mu.Lock()
	now := time.Now()
	m.pruneSessionsLocked(now)
	if len(m.sessions) >= maxSessions {
		m.evictEarliestSessionLocked()
	}
	m.sessions[hash(value)] = now.Add(m.ttl)
	m.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: value, Path: "/", HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteStrictMode, MaxAge: int(m.ttl.Seconds()), Expires: now.Add(m.ttl)})
	return true, 0
}

func (m *Manager) Clear(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(CookieName); err == nil {
		m.mu.Lock()
		delete(m.sessions, hash(cookie.Value))
		m.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}

func (m *Manager) allow(ip string) bool {
	now := time.Now()
	cut := now.Add(-time.Minute)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneSessionsLocked(now)
	m.global = trim(m.global, cut)
	if len(m.global) >= 100 {
		return false
	}
	for key, attempts := range m.attempts {
		attempts = trim(attempts, cut)
		if len(attempts) == 0 {
			delete(m.attempts, key)
			continue
		}
		m.attempts[key] = attempts
	}
	if _, exists := m.attempts[ip]; !exists && len(m.attempts) >= maxAttemptKeys {
		return false
	}
	list := m.attempts[ip]
	if len(list) >= 5 {
		return false
	}
	m.attempts[ip] = append(list, now)
	m.global = append(m.global, now)
	return true
}

func (m *Manager) pruneSessionsLocked(now time.Time) {
	for key, expiry := range m.sessions {
		if !now.Before(expiry) {
			delete(m.sessions, key)
		}
	}
}

func (m *Manager) evictEarliestSessionLocked() {
	var earliestKey string
	var earliestExpiry time.Time
	for key, expiry := range m.sessions {
		if earliestKey == "" || expiry.Before(earliestExpiry) {
			earliestKey = key
			earliestExpiry = expiry
		}
	}
	if earliestKey != "" {
		delete(m.sessions, earliestKey)
	}
}
func trim(in []time.Time, cut time.Time) []time.Time {
	n := 0
	for _, t := range in {
		if t.After(cut) {
			in[n] = t
			n++
		}
	}
	return in[:n]
}
func hash(v string) string { sum := sha256.Sum256([]byte(v)); return string(sum[:]) }
func remoteIP(r *http.Request, trusted []string) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	for _, proxy := range trusted {
		if proxy == host {
			if forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ","); len(forwarded) > 0 {
				candidate := strings.TrimSpace(forwarded[0])
				if net.ParseIP(candidate) != nil {
					return candidate
				}
			}
		}
	}
	return host
}

// LoopbackHostOnly prevents DNS-rebinding access when console authentication is
// intentionally disabled on a loopback listener.
func LoopbackHostOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if parsed, _, err := net.SplitHostPort(host); err == nil {
			host = parsed
		} else {
			host = strings.Trim(host, "[]")
		}
		host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			http.Error(w, http.StatusText(http.StatusMisdirectedRequest), http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}
