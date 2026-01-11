package oauth

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type callbackServer struct {
	server *http.Server
	port   int
}

func (s *callbackServer) start(expectedState string, codeChan chan<- string, errChan chan<- error) {
	done := make(chan struct{})

	mux := http.NewServeMux()

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		defer close(done)

		// verify state for CSRF protection
		state := r.URL.Query().Get("state")
		if state != expectedState {
			errChan <- fmt.Errorf("invalid state parameter")
			http.Error(w, "Invalid state parameter", http.StatusUnauthorized)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no authorization code received")
			http.Error(w, "No authorization code", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
			<!DOCTYPE html>
			<html>
			<head><title>Authorization Successful</title></head>
			<body style="font-family: Arial, sans-serif; text-align: center; padding: 50px;">
				<h1 style="color: #28a745;">Authorization Successful</h1>
				<p>You can close this window and return to the terminal.</p>
			</body>
			</html>
		`)

		codeChan <- code
	})

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	shutdownErrChan := make(chan error, 1)
	go func() {
		// graceful shutdown when code is received or error occurs
		<-done

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		shutdownErrChan <- s.server.Shutdown(ctx)
	}()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		errChan <- err
	}

	err := <-shutdownErrChan
	if err != nil {
		errChan <- err
	}
}
