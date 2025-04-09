package cookies

import (
	"github.com/charmbracelet/log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
)

type PersistentAuthJar struct {
	jar        *cookiejar.Jar
	filename   string
	cookieName string
	cookieURL  *url.URL
}

func NewPersistentAuthJar(filename string, cookieName string, cookieURL *url.URL) (*PersistentAuthJar, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	p := &PersistentAuthJar{
		jar:        jar,
		filename:   filename,
		cookieName: cookieName,
		cookieURL:  cookieURL,
	}

	dir := filepath.Dir(p.filename)
	err = os.MkdirAll(dir, 0700)
	if err != nil {
		log.Error("Unable create directories", "dir", dir, "err", err)
		return nil, err
	}
	bytes, err := os.ReadFile(p.filename)
	if err != nil {
		if os.IsNotExist(err) {
			log.Warn("Cookie file does not exist", "filename", p.filename)
		} else {
			log.Error("Unable to read cookie file", "filename", p.filename, "err", err)
			return nil, err
		}
	}
	p.ResetAuthCookie(string(bytes))
	return p, nil
}

func (p *PersistentAuthJar) Cookies(u *url.URL) []*http.Cookie {
	return p.jar.Cookies(u)
}

func (p *PersistentAuthJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	p.jar.SetCookies(u, cookies)
	if u.Host == p.cookieURL.Host {
		for _, cookie := range cookies {
			if cookie.Name == p.cookieName {
				log.Info("Auth cookie was updated", "exp", cookie.Expires,
					"len", len(cookie.Value), "value", head(cookie.Value)+"...")
				err := os.WriteFile(p.filename, []byte(cookie.Value), 0600)
				if err != nil {
					log.Error("Unable to save cookie file", "filename", p.filename, "err", err)
				}
				break
			}
		}
	}
}

func (p *PersistentAuthJar) ResetAuthCookie(value string) {
	aspNetAuthCookie := http.Cookie{
		Name:     p.cookieName,
		Value:    value,
		Domain:   p.cookieURL.Host,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	}
	p.SetCookies(p.cookieURL, []*http.Cookie{&aspNetAuthCookie})
}

func head(s string) string {
	if len(s) > 10 {
		return s[:10]
	}
	return s
}
