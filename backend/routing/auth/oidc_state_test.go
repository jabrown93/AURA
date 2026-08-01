package routes_auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/jwtauth/v5"
)

func withTestTokenAuth(t *testing.T) {
	t.Helper()
	previous := TokenAuth
	SetTokenAuth(jwtauth.New("HS256", []byte("test-secret-value"), nil))
	t.Cleanup(func() { TokenAuth = previous })
}

// startTx runs the login half of a sign-in and returns the transaction plus the cookie the
// browser would hold.
func startTx(t *testing.T) (*oidcTransaction, *http.Cookie) {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://aura.example.com/api/auth/oidc/login", nil)

	tx, err := startOIDCTransaction(rec, req)
	if err != nil {
		t.Fatalf("startOIDCTransaction() error = %v", err)
	}

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == oidcTxCookieName {
			return tx, cookie
		}
	}
	t.Fatal("no transaction cookie was set")
	return nil, nil
}

func callbackRequest(cookie *http.Cookie) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "http://aura.example.com/api/auth/oidc/callback", nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

func TestOIDCTransactionRoundTrip(t *testing.T) {
	withTestTokenAuth(t)

	tx, cookie := startTx(t)
	if tx.State == "" || tx.Nonce == "" || tx.Verifier == "" {
		t.Fatal("transaction is missing state, nonce or code verifier")
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Error("transaction cookie must be HttpOnly and SameSite=Lax to survive the provider redirect")
	}

	got, err := consumeOIDCTransaction(httptest.NewRecorder(), callbackRequest(cookie), tx.State)
	if err != nil {
		t.Fatalf("consumeOIDCTransaction() error = %v", err)
	}
	if got.Nonce != tx.Nonce || got.Verifier != tx.Verifier {
		t.Fatal("consumed transaction does not match the one that was issued")
	}
}

func TestOIDCTransactionRejections(t *testing.T) {
	withTestTokenAuth(t)

	t.Run("mismatched state", func(t *testing.T) {
		tx, cookie := startTx(t)
		if _, err := consumeOIDCTransaction(httptest.NewRecorder(), callbackRequest(cookie), tx.State+"x"); err == nil {
			t.Fatal("expected a state mismatch to be rejected")
		}
	})

	t.Run("empty state", func(t *testing.T) {
		_, cookie := startTx(t)
		if _, err := consumeOIDCTransaction(httptest.NewRecorder(), callbackRequest(cookie), ""); err == nil {
			t.Fatal("expected an empty state to be rejected")
		}
	})

	t.Run("no cookie", func(t *testing.T) {
		if _, err := consumeOIDCTransaction(httptest.NewRecorder(), callbackRequest(nil), "anything"); err == nil {
			t.Fatal("expected a callback without a transaction cookie to be rejected")
		}
	})

	t.Run("tampered cookie", func(t *testing.T) {
		tx, cookie := startTx(t)
		cookie.Value = cookie.Value[:len(cookie.Value)-2] + "xy"
		if _, err := consumeOIDCTransaction(httptest.NewRecorder(), callbackRequest(cookie), tx.State); err == nil {
			t.Fatal("expected a re-signed or truncated cookie to be rejected")
		}
	})

	t.Run("session token is not a transaction", func(t *testing.T) {
		// Both are signed with the same key, so the type claim is what keeps them apart.
		signed, err := IssueSessionToken("aura", nil)
		if err != nil {
			t.Fatalf("IssueSessionToken() error = %v", err)
		}
		cookie := &http.Cookie{Name: oidcTxCookieName, Value: signed}
		if _, err := consumeOIDCTransaction(httptest.NewRecorder(), callbackRequest(cookie), "anything"); err == nil {
			t.Fatal("expected a session token to be rejected as a transaction")
		}
	})
}

func TestOIDCTransactionIsSingleUse(t *testing.T) {
	withTestTokenAuth(t)

	tx, cookie := startTx(t)
	rec := httptest.NewRecorder()

	if _, err := consumeOIDCTransaction(rec, callbackRequest(cookie), tx.State); err != nil {
		t.Fatalf("consumeOIDCTransaction() error = %v", err)
	}

	// The browser is told to drop the cookie, so a replayed callback arrives without one.
	cleared := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == oidcTxCookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("the transaction cookie must be cleared once consumed")
	}
}

func TestIssuedSessionTokenIsTypedAsSession(t *testing.T) {
	withTestTokenAuth(t)

	signed, err := IssueSessionToken("aura", nil)
	if err != nil {
		t.Fatalf("IssueSessionToken() error = %v", err)
	}

	token, err := jwtauth.VerifyToken(TokenAuth, signed)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}

	var typ string
	if err := token.Get("typ", &typ); err != nil {
		t.Fatalf("token has no typ claim: %v", err)
	}
	if !IsSessionTokenType(typ) {
		t.Fatalf("typ = %q, want a session type", typ)
	}
}

func TestIsSessionTokenType(t *testing.T) {
	// Tokens issued before the claim existed carry no type and must keep working.
	if !IsSessionTokenType("") {
		t.Error("a token without a type must still be accepted")
	}
	if !IsSessionTokenType(TokenTypeSession) {
		t.Error("session tokens must be accepted")
	}
	if IsSessionTokenType(tokenTypeOIDCTx) {
		t.Error("transaction tokens must never be accepted as sessions")
	}
}
