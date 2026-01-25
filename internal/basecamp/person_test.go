package basecamp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GetMyProfile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		expectedProfile := Person{
			ID:           123,
			Name:         "Test User",
			EmailAddress: "test@example.com",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/my/profile.json", r.URL.Path)
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(expectedProfile)
		}))
		defer server.Close()

		client := newTestClient(server)
		profile, err := client.GetMyProfile(context.Background())
		require.NoError(t, err)
		require.NotNil(t, profile)

		assert.Equal(t, expectedProfile.ID, profile.ID)
		assert.Equal(t, expectedProfile.Name, profile.Name)
		assert.Equal(t, expectedProfile.EmailAddress, profile.EmailAddress)
	})

	t.Run("error from server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}))
		defer server.Close()

		client := newTestClient(server)
		profile, err := client.GetMyProfile(context.Background())
		require.Error(t, err)

		assert.Empty(t, profile)
		assert.ErrorContains(t, err, "internal error")
	})

	t.Run("context cancelled", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			json.NewEncoder(w).Encode(Person{})
		}))
		defer server.Close()

		client := newTestClient(server)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		profile, err := client.GetMyProfile(ctx)
		require.Error(t, err)

		assert.Empty(t, profile)
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json{{{"))
		}))
		defer server.Close()

		client := newTestClient(server)
		profile, err := client.GetMyProfile(context.Background())
		require.Error(t, err)

		assert.Empty(t, profile)
		assert.ErrorContains(t, err, "invalid character")
	})
}
