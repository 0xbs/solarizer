package solarweb

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"solarizer/cookies"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/sony/gobreaker/v2"
	"golang.org/x/net/html"
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
	loginURL       = "https://login.fronius.com/commonauth"
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

func (s *SolarWeb) loginSucceeded(resp *http.Response) bool {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return false
	}

	finalURL := resp.Request.URL
	if finalURL.Host != domain {
		return false
	}
	if strings.HasPrefix(finalURL.Path, "/Account/") {
		return false
	}

	for _, cookie := range s.jar.Cookies(cookieURL) {
		if cookie.Name == authCookieName && cookie.Value != "" {
			return true
		}
	}

	return false
}

func (s *SolarWeb) login() error {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()

	log.Info("Logging into SolarWeb using credentials")

	externalLoginResp, err := s.newRequest(http.MethodGet, baseURL+"/Account/ExternalLogin", nil)
	if err != nil {
		return err
	}
	defer externalLoginResp.Body.Close()

	if externalLoginResp.StatusCode < 200 || externalLoginResp.StatusCode >= 300 {
		return fmt.Errorf("external login returned %s", externalLoginResp.Status)
	}

	loginActionURL, loginValues, err := parseLoginForm(externalLoginResp, s.username, s.password)
	if err != nil {
		return err
	}

	commonauthResp, err := s.newRequest(http.MethodPost, loginActionURL, strings.NewReader(loginValues.Encode()))
	if err != nil {
		return err
	}
	defer commonauthResp.Body.Close()

	if commonauthResp.StatusCode < 200 || commonauthResp.StatusCode >= 300 {
		return fmt.Errorf("login step returned %s", commonauthResp.Status)
	}

	callbackValues, err := parseLoginCallbackForm(commonauthResp)
	if err != nil {
		return err
	}

	callbackResp, err := s.newRequest(http.MethodPost, baseURL+"/Account/ExternalLoginCallback", strings.NewReader(callbackValues.Encode()))
	if err != nil {
		return err
	}
	defer callbackResp.Body.Close()

	if !s.loginSucceeded(callbackResp) {
		return fmt.Errorf("authentication callback did not establish a SolarWeb session")
	}
	if callbackResp.StatusCode < 200 || callbackResp.StatusCode >= 300 {
		return fmt.Errorf("authentication callback returned %s", callbackResp.Status)
	}

	log.Info("SolarWeb login completed")
	return nil
}

func (s *SolarWeb) newRequest(method string, rawURL string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func parseLoginForm(resp *http.Response, username string, password string) (string, url.Values, error) {
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("unable to parse SolarWeb login form: %w", err)
	}

	form := findLoginForm(doc)
	if form == nil {
		return "", nil, fmt.Errorf("unable to find SolarWeb login form in %q", responseURL(resp))
	}

	values := url.Values{}
	collectInputValues(form, values)

	if values.Get("sessionDataKey") == "" && resp != nil && resp.Request != nil && resp.Request.URL != nil {
		values.Set("sessionDataKey", resp.Request.URL.Query().Get("sessionDataKey"))
	}
	if values.Get("sessionDataKey") == "" {
		return "", nil, fmt.Errorf("missing sessionDataKey in %q", responseURL(resp))
	}

	values.Set("username", username)
	values.Set("usernameUserInput", username)
	values.Set("password", password)
	values.Set("chkRemember", "on")

	return formActionURL(resp, form), values, nil
}

func parseLoginCallbackForm(resp *http.Response) (url.Values, error) {
	requiredFields := []string{"code", "id_token", "state", "AuthenticatedIdPs", "session_state"}
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to parse SolarWeb login callback form: %w", err)
	}

	values := url.Values{}
	for _, name := range requiredFields {
		value, ok := findHiddenInputValue(doc, name)
		if !ok {
			return nil, fmt.Errorf("unable to find hidden input %q in SolarWeb login response", name)
		}
		values.Set(name, value)
	}

	return values, nil
}

func findLoginForm(node *html.Node) *html.Node {
	if node.Type == html.ElementNode && node.Data == "form" {
		id, _ := attrValue(node, "id")
		action, _ := attrValue(node, "action")
		if id == "loginForm" || strings.Contains(action, "commonauth") {
			return node
		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if form := findLoginForm(child); form != nil {
			return form
		}
	}

	return nil
}

func collectInputValues(node *html.Node, values url.Values) {
	if node.Type == html.ElementNode && node.Data == "input" {
		name, ok := attrValue(node, "name")
		if ok && name != "" {
			inputType, _ := attrValue(node, "type")
			switch strings.ToLower(inputType) {
			case "button", "file", "image", "submit":
			case "checkbox", "radio":
				if _, checked := attrValue(node, "checked"); checked {
					value, _ := attrValue(node, "value")
					if value == "" {
						value = "on"
					}
					values.Set(name, value)
				}
			default:
				value, _ := attrValue(node, "value")
				values.Set(name, value)
			}
		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		collectInputValues(child, values)
	}
}

func formActionURL(resp *http.Response, form *html.Node) string {
	action, ok := attrValue(form, "action")
	if !ok || action == "" {
		return loginURL
	}

	actionURL, err := url.Parse(action)
	if err != nil {
		return loginURL
	}
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return actionURL.String()
	}

	return resp.Request.URL.ResolveReference(actionURL).String()
}

func responseURL(resp *http.Response) string {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return ""
	}
	return resp.Request.URL.String()
}

func attrValue(node *html.Node, targetKey string) (string, bool) {
	for _, attr := range node.Attr {
		if attr.Key == targetKey {
			return attr.Val, true
		}
	}
	return "", false
}

func findHiddenInputValue(node *html.Node, targetName string) (string, bool) {
	if node.Type == html.ElementNode && node.Data == "input" {
		var name string
		var value string
		for _, attr := range node.Attr {
			switch attr.Key {
			case "name":
				name = attr.Val
			case "value":
				value = attr.Val
			}
		}
		if name == targetName {
			return value, true
		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if value, ok := findHiddenInputValue(child, targetName); ok {
			return value, true
		}
	}

	return "", false
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
