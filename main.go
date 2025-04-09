package main

import (
	"context"
	"github.com/charmbracelet/log"
	"os"
	"os/signal"
	"solarizer/apiserver"
	"solarizer/influx"
	"solarizer/solarweb"
	"syscall"
	"time"
)

const (
	apiServerAddr = ":8080"
)

var solarWebClient *solarweb.SolarWeb

// MustGetenv retrieves the environment variable or terminates the application if not present or empty
func MustGetenv(key string) string {
	env := os.Getenv(key)
	if env == "" {
		log.Fatal("Environment variable not set", "name", key)
	}
	return env
}

func main() {
	log.Info("Starting up")

	// Initialize SolarWeb client
	pvSystemId := MustGetenv("SOLAR_WEB_PV_SYSTEM_ID")
	authCookieFilename := os.Getenv("SOLAR_WEB_AUTH_COOKIE_FILE")
	if authCookieFilename == "" {
		authCookieFilename = "/tmp/solarizer/authcookie"
	}
	solarWebClient = solarweb.New(pvSystemId, authCookieFilename)
	if authCookie, ok := os.LookupEnv("SOLAR_WEB_AUTH_COOKIE"); ok {
		solarWebClient.SetAuthCookie(authCookie)
	}
	log.Info("SolarWeb client initialized", "pvSystemId", pvSystemId)

	// Create importer
	dbConfig := influx.DBConfig{
		Url:    MustGetenv("INFLUX_URL"),
		Token:  MustGetenv("INFLUX_TOKEN"),
		Org:    MustGetenv("INFLUX_ORG"),
		Bucket: MustGetenv("INFLUX_BUCKET"),
	}
	importer := influx.NewImporter(dbConfig, solarWebClient)
	log.Info("Influx importer initialized")

	// Create api
	api := apiserver.New(apiServerAddr, solarWebClient)
	log.Info("API server initialized", "addr", apiServerAddr)

	// Create a channel to capture SIGTERM, SIGINT
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	// Create a context listening to SIGTERM, SIGINT
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Run tasks asynchronously
	go api.ListenAndServe()
	go importer.RunImportLoop(ctx)

	// Block and wait for signal
	sig := <-quit
	log.Info("Shutting down", "signal", sig)

	// Shutdown api server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := api.Shutdown(shutdownCtx); err != nil {
		log.Error("Shutdown of API server failed", "err", err)
	}

	log.Info("Shutdown complete")
}
