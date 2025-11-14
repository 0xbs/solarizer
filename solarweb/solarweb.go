package solarweb

import (
	"encoding/json"
	"fmt"
	"github.com/sony/gobreaker/v2"
	"net/http"
	"net/url"
	"solarizer/cookies"
	"time"
)

// Better Login:
//https://github.com/mattsmith24/pictureframe/blob/f22730227e56c0067d86288c9afe90ca20d1352a/solarweb.py#L124

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

type SolarWeb struct {
	pvSystemId string
	jar        *cookies.PersistentAuthJar
	cb         *gobreaker.CircuitBreaker[*http.Response]
	client     *http.Client
}

func New(pvSystemId string, authCookieFilename string) *SolarWeb {
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
	}
	cb := gobreaker.NewCircuitBreaker[*http.Response](cbSettings)

	s := &SolarWeb{
		pvSystemId: pvSystemId,
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
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("received non successful status code %s", resp.Status)
		}
		return resp, nil
	})

	if err != nil {
		return nil, err
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
