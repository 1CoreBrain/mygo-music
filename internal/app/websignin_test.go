package app

import (
	"testing"

	"github.com/egoist/mygo"
)

// A sign-in page is done once every cookie the plugin named is there, and not
// for one with nothing in it, as a site empties one to take it back; it then
// answers all it has as a Cookie header.
func TestSignInPageWaitsForEveryCookie(t *testing.T) {
	until := []string{"SAPISID", "__Secure-3PSID"}
	cookies := []mygo.Cookie{{Name: "SAPISID", Value: "a"}, {Name: "__Secure-3PSID", Value: ""}, {Name: "NID", Value: "n"}}
	if _, ok := cookieHeader(cookies, until); ok {
		t.Error("done before every cookie was there")
	}
	cookies[1].Value = "b"
	if header, ok := cookieHeader(cookies, until); !ok || header != "SAPISID=a; __Secure-3PSID=b; NID=n" {
		t.Errorf("answered %q, %v", header, ok)
	}
}
