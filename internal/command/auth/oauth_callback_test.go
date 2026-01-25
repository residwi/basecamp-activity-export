package auth

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallbackServer(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		srv := &callbackServer{port: getFreePort(t)}

		expectedState := "test-state-12345"
		expectedCode := "auth-code-xyz"

		codeChan := make(chan string, 1)
		errChan := make(chan error, 1)

		go srv.start(expectedState, codeChan, errChan)

		callbackURL := fmt.Sprintf("http://localhost:%d/callback?state=%s&code=%s", srv.port, url.QueryEscape(expectedState), url.QueryEscape(expectedCode))
		resp, err := http.Get(callbackURL)
		if err != nil {
			t.Skipf("Server failed to respond: %v", err)
		}
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "expected HTTP 200")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "failed to read response body")

		bodyStr := string(body)
		assert.Contains(t, bodyStr, "Authorization Successful", "expected success message")

		code := <-codeChan
		assert.Equal(t, expectedCode, code, "expected correct authorization code")

		select {
		case err := <-errChan:
			t.Errorf("unexpected error: %v", err)
		default:
			// expected no error
		}
	})

	t.Run("invalid state", func(t *testing.T) {
		srv := &callbackServer{port: getFreePort(t)}
		expectedState := "correct-state"
		wrongState := "wrong-state"

		codeChan := make(chan string, 1)
		errChan := make(chan error, 1)

		go srv.start(expectedState, codeChan, errChan)

		callbackURL := fmt.Sprintf("http://localhost:%d/callback?state=%s&code=test-code",
			srv.port, url.QueryEscape(wrongState))

		resp, err := http.Get(callbackURL)
		if err != nil {
			t.Skipf("Server failed to respond: %v", err)
		}
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "expected HTTP 401 for invalid state")

		errResponse := <-errChan
		assert.EqualError(t, errResponse, "invalid state parameter")

		select {
		case code := <-codeChan:
			t.Errorf("unexpected code received: %s", code)
		default:
			// expected no code
		}
	})

	t.Run("missing code", func(t *testing.T) {
		srv := &callbackServer{port: getFreePort(t)}

		expectedState := "some-state"
		codeChan := make(chan string, 1)
		errChan := make(chan error, 1)

		go srv.start(expectedState, codeChan, errChan)

		callbackURL := fmt.Sprintf("http://localhost:%d/callback?state=%s",
			srv.port, url.QueryEscape(expectedState))
		resp, err := http.Get(callbackURL)
		if err != nil {
			t.Skipf("Server failed to respond: %v", err)
		}
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "expected HTTP 400 for missing code")

		errResponse := <-errChan
		assert.EqualError(t, errResponse, "no authorization code received")

		select {
		case code := <-codeChan:
			t.Errorf("unexpected code received: %s", code)
		default:
			// expected no code
		}
	})
}

func getFreePort(t *testing.T) int {
	addr, _ := net.ResolveTCPAddr("tcp", "localhost:0")
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}

	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port
}
