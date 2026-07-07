package solarweb

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"solarizer/cookies"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/sony/gobreaker/v2"
)

// Endpoints:
// /ActualData/GetCompareDataForPvSystem?pvSystemId={pvSystemId}
// /Chart/GetWidgetChart?PvSystemId={pvSystemId}
// /Messages/GetUnreadMessageCountForUser
// /Messages/GetUnreadMessages
// /PvSystemImages/GetUrlForId?PvSystemId={pvSystemId}
// /PvSystems/GetPvSystemProductionsAndEarnings?pvSystemId={pvSystemId}
// /PvSystems/GetWeatherWidgetData?pvSystemId={pvSystemId}

const (
	domain         = "www.solarweb.com"
	baseURL        = "https://" + domain
	timeout        = 10 * time.Second
	userAgent      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"
	authCookieName = ".AspNet.Auth"
)

var cookieURL = &url.URL{Scheme: "https", Host: domain, Path: "/"}
var errAuthenticationRequired = errors.New("authentication required")

type SolarWeb struct {
	pvSystemId string
	username   string
	password   string
	jar        *cookies.PersistentAuthJar
	cb         *gobreaker.CircuitBreaker[*http.Response]
	client     *http.Client
	loginMu    sync.Mutex
}

func New(pvSystemId string, authCookieFilename string, username string, password string) *SolarWeb {
	// Create a cookie jar that stores the initial and updated auth cookies
	jar, err := cookies.NewPersistentAuthJar(authCookieFilename, authCookieName, cookieURL)
	if err != nil {
		panic(err)
	}

	cbSettings := gobreaker.Settings{
		Name: "SolarWeb",
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		IsExcluded: func(err error) bool {
			return errors.Is(err, errAuthenticationRequired)
		},
	}
	cb := gobreaker.NewCircuitBreaker[*http.Response](cbSettings)

	s := &SolarWeb{
		pvSystemId: pvSystemId,
		username:   username,
		password:   password,
		jar:        jar,
		cb:         cb,
		client: &http.Client{
			Jar:     jar,
			Timeout: timeout,
		},
	}
	return s
}

func (s *SolarWeb) SetAuthCookie(value string) {
	s.jar.ResetAuthCookie(value)
}

func (s *SolarWeb) get(path string) (*http.Response, error) {
	resp, err := s.doGet(path)
	if err == nil {
		return resp, nil
	}

	if !errors.Is(err, errAuthenticationRequired) {
		return nil, err
	}

	log.Warn("SolarWeb authentication required, attempting re-authentication", "path", path)
	if loginErr := s.login(); loginErr != nil {
		return nil, fmt.Errorf("%w: automatic re-login failed: %w", err, loginErr)
	}

	return s.doGet(path)
}

func (s *SolarWeb) doGet(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.cb.Execute(func() (*http.Response, error) {
		resp, httpErr := s.client.Do(req)
		if httpErr != nil {
			return nil, httpErr
		}
		if s.isAuthenticationRequired(resp) {
			_ = resp.Body.Close()
			return nil, errAuthenticationRequired
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("received non successful status code %s", resp.Status)
		}
		return resp, nil
	})

	if err != nil {
		return nil, err
	}

	return resp, err
}

func (s *SolarWeb) isAuthenticationRequired(resp *http.Response) bool {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return false
	}

	finalURL := resp.Request.URL
	if finalURL.Host != domain {
		return true
	}

	if strings.HasPrefix(finalURL.Path, "/Account/") {
		return true
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	return strings.Contains(contentType, "text/html")
}

func (s *SolarWeb) GetCompareData() (CompareData, error) {
	var data CompareData

	resp, err := s.get("/ActualData/GetCompareDataForPvSystem?pvSystemId=" + s.pvSystemId)
	if err != nil {
		return data, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&data)
	return data, err
}

func (s *SolarWeb) GetProductionsAndEarnings() (ProductionsAndEarnings, error) {
	var data ProductionsAndEarnings

	resp, err := s.get("/PvSystems/GetPvSystemProductionsAndEarnings?pvSystemId=" + s.pvSystemId)
	if err != nil {
		return data, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&data)
	return data, err
}

func (s *SolarWeb) GetWidgetChart() (WidgetChart, error) {
	var data WidgetChart

	resp, err := s.get("/Chart/GetWidgetChart?PvSystemId=" + s.pvSystemId)
	if err != nil {
		return data, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&data)
	return data, err
}
