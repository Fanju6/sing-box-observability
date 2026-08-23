package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginCookieExpiryAndConstantTimePath(t *testing.T) {
	m := New("secret", time.Minute, true)
	r := httptest.NewRequest("POST", "/", nil)
	r.RemoteAddr = "192.0.2.1:1234"
	w := httptest.NewRecorder()
	ok, _ := m.Login(w, r, "secret")
	if !ok {
		t.Fatal("login rejected")
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie %#v", cookie)
	}
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.AddCookie(cookie)
	if !m.Authenticated(r2) {
		t.Fatal("session not accepted")
	}
	m.mu.Lock()
	for key := range m.sessions {
		m.sessions[key] = time.Now().Add(-time.Second)
	}
	m.mu.Unlock()
	if m.Authenticated(r2) {
		t.Fatal("expired session accepted")
	}
}

func TestLoginRateLimit(t *testing.T) {
	m := New("secret", time.Minute, false)
	for i := 0; i < 5; i++ {
		r := httptest.NewRequest("POST", "/", nil)
		r.RemoteAddr = "192.0.2.2:1"
		ok, _ := m.Login(httptest.NewRecorder(), r, "bad")
		if ok {
			t.Fatal("bad token accepted")
		}
	}
	r := httptest.NewRequest("POST", "/", nil)
	r.RemoteAddr = "192.0.2.2:1"
	ok, retry := m.Login(httptest.NewRecorder(), r, "secret")
	if ok || retry <= 0 {
		t.Fatalf("rate limit not applied ok=%v retry=%v", ok, retry)
	}
}

func TestClearRevokesServerSession(t *testing.T) {
	m := New("secret", time.Minute, false)
	loginRequest := httptest.NewRequest("POST", "/", nil)
	loginRequest.RemoteAddr = "192.0.2.3:1234"
	loginResponse := httptest.NewRecorder()
	ok, _ := m.Login(loginResponse, loginRequest, "secret")
	if !ok {
		t.Fatal("login rejected")
	}
	cookie := loginResponse.Result().Cookies()[0]
	request := httptest.NewRequest("DELETE", "/", nil)
	request.AddCookie(cookie)
	m.Clear(httptest.NewRecorder(), request)
	if m.Authenticated(request) {
		t.Fatal("cleared session is still accepted")
	}
}

func TestExpiredAuthenticationStateIsPruned(t *testing.T) {
	m := New("secret", time.Minute, false)
	m.mu.Lock()
	m.sessions["expired"] = time.Now().Add(-time.Second)
	m.attempts["192.0.2.4"] = []time.Time{time.Now().Add(-2 * time.Minute)}
	m.mu.Unlock()

	request := httptest.NewRequest("POST", "/", nil)
	request.RemoteAddr = "192.0.2.5:1234"
	m.Login(httptest.NewRecorder(), request, "bad")

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.sessions["expired"]; exists {
		t.Fatal("expired session was not pruned")
	}
	if _, exists := m.attempts["192.0.2.4"]; exists {
		t.Fatal("expired rate-limit key was not pruned")
	}
}

func TestLoopbackHostOnly(t *testing.T) {
	handler := LoopbackHostOnly(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, host := range []string{"localhost:9095", "localhost.", "127.0.0.1:9095", "[::1]:9095"} {
		request := httptest.NewRequest("GET", "http://"+host+"/healthz", nil)
		request.Host = host
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("loopback host %q returned %d", host, response.Code)
		}
	}
	for _, host := range []string{"example.com", "0.0.0.0:9095", "192.0.2.10:9095"} {
		request := httptest.NewRequest("GET", "http://127.0.0.1/healthz", nil)
		request.Host = host
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusMisdirectedRequest {
			t.Fatalf("non-loopback host %q returned %d", host, response.Code)
		}
	}
}
