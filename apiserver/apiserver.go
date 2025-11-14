package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"solarizer/solarweb"
	"strings"

	"github.com/charmbracelet/log"
)

type ApiServer struct {
	server         *http.Server
	apiTokens      map[string]bool
	solarWebClient *solarweb.SolarWeb
}

func New(addr string, solarWebClient *solarweb.SolarWeb) *ApiServer {
	// Create server
	mux := http.NewServeMux()

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	s := &ApiServer{
		server:         server,
		apiTokens:      make(map[string]bool),
		solarWebClient: solarWebClient,
	}

	mux.HandleFunc("/api/auth/cookie", s.putAuthCookie)
	mux.HandleFunc("/api/pv/power", s.getPowerData)
	mux.HandleFunc("/api/pv/production", s.getProductionsAndEarnings)
	mux.HandleFunc("/api/pv/balance", s.getBalance)

	s.initApiTokens()

	return s
}

func (s *ApiServer) ListenAndServe() {
	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("ListenAndServe error", "err", err)
	}
}

func (s *ApiServer) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *ApiServer) initApiTokens() {
	apiTokenEnv, ok := os.LookupEnv("API_TOKENS")
	if !ok {
		log.Fatal("Environment variable API_TOKENS not set")
	}
	apiTokenList := strings.Split(apiTokenEnv, ",")
	for _, apiToken := range apiTokenList {
		s.apiTokens[apiToken] = true
	}
}

func (s *ApiServer) validateApiToken(r *http.Request) (error, int) {
	const prefix = "Bearer "
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, prefix) {
		return fmt.Errorf("empty or invalid authorization header"), http.StatusUnauthorized
	}
	authToken := authHeader[len(prefix):]
	if authToken == "" {
		return fmt.Errorf("empty or invalid API key"), http.StatusUnauthorized
	}
	if !s.apiTokens[authToken] {
		return fmt.Errorf("unknown API key"), http.StatusForbidden
	}
	return nil, 0
}

func (s *ApiServer) putAuthCookie(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
	if err, status := s.validateApiToken(r); err != nil {
		log.Warn("Error validating API token", "err", err)
		http.Error(w, "", status)
		return
	}
	log.Debug("Received putAuthCookie request")
	authCookieBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.solarWebClient.SetAuthCookie(string(authCookieBytes))
	w.WriteHeader(http.StatusAccepted)
}

func (s *ApiServer) getPowerData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
	if err, status := s.validateApiToken(r); err != nil {
		log.Warn("Error validating API token", "err", err)
		http.Error(w, "", status)
		return
	}
	log.Debug("Received getPowerData request")
	data, err := s.solarWebClient.GetCompareData()
	if err != nil {
		log.Error("Error requesting power data", "err", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(data)
	if err != nil {
		log.Error("Error encoding to JSON", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *ApiServer) getProductionsAndEarnings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
	if err, status := s.validateApiToken(r); err != nil {
		log.Warn("Error validating API token", "err", err)
		http.Error(w, "", status)
		return
	}
	log.Debug("Received getProductionsAndEarnings request")
	data, err := s.solarWebClient.GetProductionsAndEarnings()
	if err != nil {
		log.Error("Error requesting earnings data", "err", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(data)
	if err != nil {
		log.Error("Error encoding to JSON", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *ApiServer) getBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
	if err, status := s.validateApiToken(r); err != nil {
		log.Warn("Error validating API token", "err", err)
		http.Error(w, "", status)
		return
	}
	log.Debug("Received getBalance request")
	data, err := s.solarWebClient.GetWidgetChart()
	if err != nil {
		log.Error("Error requesting balance data", "err", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(data)
	if err != nil {
		log.Error("Error encoding to JSON", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
