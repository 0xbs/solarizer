package solarweb

import (
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/log"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"
)

// Endpoints:
// /ActualData/GetCompareDataForPvSystem?pvSystemId={pvSystemId}
// /Chart/GetWidgetChart?PvSystemId={pvSystemId}
// /Messages/GetUnreadMessageCountForUser
// /Messages/GetUnreadMessages
// /PvSystemImages/GetUrlForId?PvSystemId={pvSystemId}
// /PvSystems/GetPvSystemEarningsAndSavings?pvSystemId={pvSystemId}
// /PvSystems/GetWeatherWidgetData?pvSystemId={pvSystemId}

const (
	domain         = "www.solarweb.com"
	baseURL        = "https://" + domain
	timeout        = 10 * time.Second
	userAgent      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"
	authCookieName = ".AspNet.Auth"
)

var cookieURL = &url.URL{Scheme: "https", Host: domain, Path: "/"}

type SolarWeb struct {
	ready          bool
	pvSystemId     string
	lastAuthCookie string
	jar            *cookiejar.Jar
	client         *http.Client
}

func New(pvSystemId string) *SolarWeb {
	// Create a cookie jar that stores the initial and updated auth cookies
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	s := &SolarWeb{
		ready:      false,
		pvSystemId: pvSystemId,
		jar:        jar,
		client: &http.Client{
			Jar:     jar,
			Timeout: timeout,
		},
	}
	return s
}

func (s *SolarWeb) IsReady() bool {
	return s.ready
}

func (s *SolarWeb) GetAuthCookie() string {
	cookie := s.findAuthCookieInJar()
	if cookie != nil {
		return cookie.Value
	}
	return ""
}

func (s *SolarWeb) findAuthCookieInJar() *http.Cookie {
	for _, cookie := range s.jar.Cookies(cookieURL) {
		if cookie.Name == authCookieName {
			return cookie
		}
	}
	return nil
}

func (s *SolarWeb) SetAuthCookie(authCookie string) {
	aspNetAuthCookie := http.Cookie{
		Name:     authCookieName,
		Value:    authCookie,
		Domain:   domain,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	}
	s.jar.SetCookies(cookieURL, []*http.Cookie{&aspNetAuthCookie})
	s.lastAuthCookie = authCookie
	s.ready = true
}

func head(s string) string {
	if len(s) > 10 {
		return s[:10]
	}
	return s
}

func (s *SolarWeb) get(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		s.ready = false
		defer resp.Body.Close()
		body := ""
		bodyBytes, bodyErr := io.ReadAll(resp.Body)
		if bodyErr == nil {
			body = string(bodyBytes)
		}
		return nil, fmt.Errorf("received non successful status code %d %s %s", resp.StatusCode, resp.Status, body)
	}

	// Check if auth cookie changed (could also parse Set-Cookie header)
	authCookie := s.findAuthCookieInJar()
	if s.lastAuthCookie != authCookie.Value {
		log.Info("Auth cookie changed", "exp", authCookie.Expires,
			"len", len(authCookie.Value), "value", head(authCookie.Value))
		s.lastAuthCookie = authCookie.Value
	}

	return resp, err
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

func (s *SolarWeb) GetEarningsAndSavings() (EarningAndSavings, error) {
	var data EarningAndSavings

	resp, err := s.get("/PvSystems/GetPvSystemEarningsAndSavings?pvSystemId=" + s.pvSystemId)
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
