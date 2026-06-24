package solarweb

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestParseLoginForm(t *testing.T) {
	resp := loginFormResponse(t, `
		<form action="../commonauth" method="post" id="loginForm">
			<input type="hidden" name="authenticators" value="SAMLSSOAuthenticator:Fronius Login;FroniusBasicAuthenticator:LOCAL">
			<input type="hidden" name="tenantDomain" value="carbon.super">
			<input type="hidden" name="allLoginParams" value="client_id=abc&sessionDataKey=form-key">
			<input id="username" name="username" type="hidden" value="null">
			<input type="text" name="usernameUserInput" id="usernameUserInput" value="">
			<input type="password" id="password" name="password" value="">
			<input class="fro-checkbox" type="checkbox" id="chkRemember" name="chkRemember">
			<input type="hidden" name="sessionDataKey" value="form-key">
			<button type="submit">Continue</button>
		</form>
	`)

	actionURL, values, err := parseLoginForm(resp, "user@example.com", "secret")
	if err != nil {
		t.Fatalf("parseLoginForm returned error: %v", err)
	}

	if actionURL != "https://login.fronius.com/commonauth" {
		t.Fatalf("unexpected action URL: %q", actionURL)
	}
	assertFormValue(t, values, "sessionDataKey", "form-key")
	assertFormValue(t, values, "username", "user@example.com")
	assertFormValue(t, values, "usernameUserInput", "user@example.com")
	assertFormValue(t, values, "password", "secret")
	assertFormValue(t, values, "chkRemember", "on")
	assertFormValue(t, values, "authenticators", "SAMLSSOAuthenticator:Fronius Login;FroniusBasicAuthenticator:LOCAL")
}

func TestParseLoginFormFallsBackToSessionDataKeyFromURL(t *testing.T) {
	resp := loginFormResponse(t, `
		<form action="../commonauth" method="post" id="loginForm">
			<input id="username" name="username" type="hidden" value="null">
			<input type="password" id="password" name="password" value="">
		</form>
	`)

	_, values, err := parseLoginForm(resp, "user@example.com", "secret")
	if err != nil {
		t.Fatalf("parseLoginForm returned error: %v", err)
	}

	assertFormValue(t, values, "sessionDataKey", "url-key")
}

func TestParseLoginFormRequiresLoginForm(t *testing.T) {
	resp := loginFormResponse(t, `<html><body>No login form</body></html>`)

	_, _, err := parseLoginForm(resp, "user@example.com", "secret")
	if err == nil {
		t.Fatal("parseLoginForm returned nil error")
	}
	if !strings.Contains(err.Error(), "unable to find SolarWeb login form") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func loginFormResponse(t *testing.T, body string) *http.Response {
	t.Helper()

	reqURL, err := url.Parse("https://login.fronius.com/authenticationendpoint/login.do?sessionDataKey=url-key")
	if err != nil {
		t.Fatal(err)
	}

	return &http.Response{
		Request: &http.Request{URL: reqURL},
		Body:    io.NopCloser(strings.NewReader(body)),
	}
}

func assertFormValue(t *testing.T, values url.Values, key string, want string) {
	t.Helper()

	if got := values.Get(key); got != want {
		t.Fatalf("values[%q] = %q, want %q", key, got, want)
	}
}
