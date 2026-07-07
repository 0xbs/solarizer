package solarweb

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/charmbracelet/log"
	"golang.org/x/net/html"
)

// Better Login:
// https://github.com/mattsmith24/pictureframe/blob/f22730227e56c0067d86288c9afe90ca20d1352a/solarweb.py#L124

// loginURL is the fallback form action used by the Fronius login page.
const loginURL = "https://login.fronius.com/commonauth"

// loginSucceeded verifies that the SolarWeb callback established a usable
// www.solarweb.com session and that the persistent auth cookie is present.
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

// login refreshes the SolarWeb session by walking the Fronius OIDC flow.
// Depending on the current Fronius session state, the first response may be
// either a username/password form or an already completed OIDC callback form.
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

	loginActionURL, loginValues, callbackValues, err := parseLoginStartResponse(externalLoginResp, s.username, s.password)
	if err != nil {
		return err
	}

	if callbackValues == nil {
		commonauthResp, err := s.newRequest(http.MethodPost, loginActionURL, strings.NewReader(loginValues.Encode()))
		if err != nil {
			return err
		}
		defer commonauthResp.Body.Close()

		if commonauthResp.StatusCode < 200 || commonauthResp.StatusCode >= 300 {
			return fmt.Errorf("login step returned %s", commonauthResp.Status)
		}

		callbackValues, err = parseLoginCallbackForm(commonauthResp)
		if err != nil {
			return err
		}
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

// newRequest sends one request in the HTML-based login flow with browser-like
// headers. JSON API calls use doGet instead.
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

// parseLoginStartResponse interprets the response from /Account/ExternalLogin.
// It returns either values for the next commonauth login POST, or callback
// values that can be posted directly to SolarWeb's ExternalLoginCallback.
func parseLoginStartResponse(resp *http.Response, username string, password string) (string, url.Values, url.Values, error) {
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return "", nil, nil, fmt.Errorf("unable to parse SolarWeb login response: %w", err)
	}

	if hasLoginCallbackForm(doc) {
		callbackValues, err := loginCallbackValuesFromDocument(doc)
		if err != nil {
			return "", nil, nil, err
		}
		return "", nil, callbackValues, nil
	}

	loginActionURL, loginValues, err := loginFormValuesFromDocument(resp, doc, username, password)
	if err != nil {
		return "", nil, nil, err
	}

	return loginActionURL, loginValues, nil, nil
}

// parseLoginForm extracts the Fronius username/password form from a response.
// It is the narrow parser used by tests and by parseLoginStartResponse after
// the response has been classified as a real login form.
func parseLoginForm(resp *http.Response, username string, password string) (string, url.Values, error) {
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("unable to parse SolarWeb login form: %w", err)
	}

	return loginFormValuesFromDocument(resp, doc, username, password)
}

// loginFormValuesFromDocument collects the existing hidden form fields,
// injects the configured credentials, and resolves the form action URL.
func loginFormValuesFromDocument(resp *http.Response, doc *html.Node, username string, password string) (string, url.Values, error) {
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

// parseLoginCallbackForm extracts the hidden OIDC callback fields returned by
// Fronius after successful authentication.
func parseLoginCallbackForm(resp *http.Response) (url.Values, error) {
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to parse SolarWeb login callback form: %w", err)
	}

	return loginCallbackValuesFromDocument(doc)
}

// loginCallbackValuesFromDocument validates and collects the fields required
// by SolarWeb's /Account/ExternalLoginCallback endpoint.
func loginCallbackValuesFromDocument(doc *html.Node) (url.Values, error) {
	requiredFields := []string{"code", "id_token", "state", "AuthenticatedIdPs", "session_state"}
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

// hasLoginCallbackForm cheaply detects a completed OIDC callback form. This
// lets the login flow skip the credential POST when Fronius already has an
// active identity-provider session.
func hasLoginCallbackForm(doc *html.Node) bool {
	if _, ok := findHiddenInputValue(doc, "code"); !ok {
		return false
	}
	if _, ok := findHiddenInputValue(doc, "id_token"); !ok {
		return false
	}
	return true
}

// findLoginForm searches the parsed HTML for the Fronius credential form.
// Fronius has used both a stable id and a commonauth action, so both are
// accepted.
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

// collectInputValues copies successful HTML input values into the form body.
// It mirrors browser form submission enough for hidden inputs and checked
// checkboxes/radios, while ignoring button-like controls.
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

// formActionURL resolves the login form's action against the response URL and
// falls back to the known commonauth endpoint if the action is absent or bad.
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

// responseURL returns the final URL for diagnostics without assuming every
// synthetic test response has a Request attached.
func responseURL(resp *http.Response) string {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return ""
	}
	return resp.Request.URL.String()
}

// attrValue returns one HTML attribute value from a node.
func attrValue(node *html.Node, targetKey string) (string, bool) {
	for _, attr := range node.Attr {
		if attr.Key == targetKey {
			return attr.Val, true
		}
	}
	return "", false
}

// findHiddenInputValue searches the parsed HTML for an input with the requested
// name and returns its value. It is used for OIDC callback fields, which are
// delivered as hidden inputs.
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
