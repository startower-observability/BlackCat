package dashboard

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/startower-observability/blackcat/internal/config"
)

func TestTokenAuthValid(t *testing.T) {
	t.Parallel()

	called := false
	handler := TokenAuthMiddleware("top-secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	req.Header.Set("Authorization", "Bearer top-secret")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !called {
		t.Fatal("expected wrapped handler to be called")
	}
}

func TestTokenAuthInvalid(t *testing.T) {
	t.Parallel()

	handler := TokenAuthMiddleware("top-secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestTokenAuthMissing(t *testing.T) {
	t.Parallel()

	handler := TokenAuthMiddleware("top-secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestDisabledDashboard(t *testing.T) {
	t.Parallel()

	cfg := config.DashboardConfig{
		Enabled: false,
		Addr:    ":8081",
		Token:   "top-secret",
	}

	server := NewServer(cfg, DashboardDeps{})
	if server != nil {
		t.Fatal("expected nil server when dashboard is disabled")
	}
}

// ---------------------------------------------------------------------------
// Integration test helpers
// ---------------------------------------------------------------------------

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	cfg := config.DashboardConfig{Enabled: true, Addr: ":0", Token: "test-token"}
	s := NewServer(cfg, DashboardDeps{})
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)
	return s, ts
}

func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// ---------------------------------------------------------------------------
// Integration tests (12)
// ---------------------------------------------------------------------------

func TestLoginPageServesSPA(t *testing.T) {
	_, ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/dashboard/login")
	if err != nil {
		t.Fatalf("GET /dashboard/login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `id="root"`) {
		t.Fatal("expected SPA body to contain id=\"root\"")
	}
}

func TestLoginSuccess(t *testing.T) {
	_, ts := newTestServer(t)
	client := noRedirectClient()

	form := url.Values{}
	form.Set("token", "test-token")
	form.Set("next", "/dashboard/")

	resp, err := client.Post(ts.URL+"/dashboard/login", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("POST /dashboard/login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}

	loc := resp.Header.Get("Location")
	if loc != "/dashboard/" {
		t.Fatalf("expected Location /dashboard/, got %q", loc)
	}

	setCookie := resp.Header.Get("Set-Cookie")
	if !strings.Contains(setCookie, "blackcat_session=") {
		t.Fatalf("expected Set-Cookie with blackcat_session, got %q", setCookie)
	}
}

func TestLoginFailure(t *testing.T) {
	_, ts := newTestServer(t)
	client := noRedirectClient()

	form := url.Values{}
	form.Set("token", "wrongtoken")

	resp, err := client.Post(ts.URL+"/dashboard/login", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("POST /dashboard/login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}

	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "/dashboard/login") {
		t.Fatalf("expected redirect to /dashboard/login, got %q", loc)
	}
	if !strings.Contains(loc, "error=invalid_token") {
		t.Fatalf("expected error=invalid_token in redirect, got %q", loc)
	}

	setCookie := resp.Header.Get("Set-Cookie")
	if strings.Contains(setCookie, "blackcat_session=") {
		t.Fatal("expected NO blackcat_session cookie on failed login")
	}
}

func TestLogout(t *testing.T) {
	_, ts := newTestServer(t)
	client := noRedirectClient()

	resp, err := client.Get(ts.URL + "/dashboard/logout")
	if err != nil {
		t.Fatalf("GET /dashboard/logout: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}

	loc := resp.Header.Get("Location")
	if loc != "/dashboard/login" {
		t.Fatalf("expected Location /dashboard/login, got %q", loc)
	}

	setCookie := resp.Header.Get("Set-Cookie")
	if !strings.Contains(setCookie, "Max-Age=0") && !strings.Contains(setCookie, "Max-Age=-1") {
		t.Fatalf("expected cookie to be cleared (Max-Age=0 or -1), got %q", setCookie)
	}
}

func TestCookieAuth(t *testing.T) {
	s, ts := newTestServer(t)
	client := noRedirectClient()

	cookieValue := signSession("test-token", s.sessionSecret)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/dashboard/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookieValue})

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /dashboard/: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestCookieAuthTampered(t *testing.T) {
	_, ts := newTestServer(t)
	client := noRedirectClient()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/dashboard/", nil)
	req.Header.Set("Accept", "text/html")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tampered:value"})

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /dashboard/: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}

	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "/dashboard/login") {
		t.Fatalf("expected redirect to login, got Location %q", loc)
	}
}

func TestBearerAuthStillWorks(t *testing.T) {
	_, ts := newTestServer(t)
	client := noRedirectClient()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/dashboard/", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /dashboard/: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestBrowserRedirectToLogin(t *testing.T) {
	_, ts := newTestServer(t)
	client := noRedirectClient()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/dashboard/", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /dashboard/: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}

	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "/dashboard/login") {
		t.Fatalf("expected redirect to /dashboard/login, got %q", loc)
	}
}

func TestAPIClientGets401(t *testing.T) {
	_, ts := newTestServer(t)
	client := noRedirectClient()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/dashboard/", nil)
	// No auth, no Accept: text/html

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /dashboard/: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"error"`) {
		t.Fatal("expected body to contain '\"error\"'")
	}
}

func TestEventsRequiresAuth(t *testing.T) {
	_, ts := newTestServer(t)
	client := noRedirectClient()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/dashboard/events", nil)
	req.Header.Set("Accept", "text/html")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /dashboard/events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}
}

func TestOpenRedirectPrevention(t *testing.T) {
	_, ts := newTestServer(t)
	client := noRedirectClient()

	form := url.Values{}
	form.Set("token", "test-token")
	form.Set("next", "//evil.com")

	resp, err := client.Post(ts.URL+"/dashboard/login", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("POST /dashboard/login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}

	loc := resp.Header.Get("Location")
	if loc != "/dashboard/" {
		t.Fatalf("expected Location /dashboard/ (open redirect prevented), got %q", loc)
	}
}

func TestSPAServedOnIndex(t *testing.T) {
	_, ts := newTestServer(t)
	client := noRedirectClient()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/dashboard/", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /dashboard/: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `id="root"`) {
		t.Fatal("expected SPA body to contain id=\"root\"")
	}
}

func TestSPAFallbackOnUnknownPath(t *testing.T) {
	_, ts := newTestServer(t)
	client := noRedirectClient()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/dashboard/nonexistent-path", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /dashboard/nonexistent-path: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `id="root"`) {
		t.Fatal("expected SPA fallback body to contain id=\"root\"")
	}
}

func TestCatStateAPI(t *testing.T) {
	_, ts := newTestServer(t)
	client := noRedirectClient()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/dashboard/api/cat-state", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /dashboard/api/cat-state: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"state"`) {
		t.Fatalf("expected JSON with state field, got %q", string(body))
	}
}
