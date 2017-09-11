package cookie

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/thejerf/abtime"
	"github.com/thejerf/sphyraena/secret"
)

type successfulTest struct {
	name   string
	value  string
	opts   []func(*OutCookie) error
	result string
}

var faketime abtime.AbstractTime

func init() {
	// Friday, 14-Jul-17 02:40:00 UTC
	faketime = abtime.NewManualAtTime(time.Unix(1500000000, 0).UTC())
	t = faketime
}

// This function tests the rendering of the cookie when it should be
// successful. It ignores HMAC, we'll test that separately.
func TestSuccessfulRendering(t *testing.T) {
	// hardcoded so the below signatures are constant.
	authenticator := secret.New([]byte("badsecret"))
	sessionID, _ := authenticator.Authenticate([]byte("session"), []byte("1"))
	sessionCookie := "session=" + string(sessionID)

	tests := []successfulTest{
		{"c", "v", []func(*OutCookie) error{}, "c=v__!sauthed!_TmVPtWyCByrJUs%HCJ5OjyPUH9UlJA5r%u1O2$nLQNg;Path=/;HttpOnly;Secure"},
		{"c", "v", []func(*OutCookie) error{Delete}, "c=;Expires=Fri, 02-Jan-1970 00:00:01 GMT;Path=/;HttpOnly;Secure"},
		// ensure order works
		{"c", "v", []func(*OutCookie) error{Duration(time.Hour), Session},
			"c=v__!sauthed!_TmVPtWyCByrJUs%HCJ5OjyPUH9UlJA5r%u1O2$nLQNg;Path=/;HttpOnly;Secure"},
		{"c", "v", []func(*OutCookie) error{Duration(time.Hour)},
			"c=v__!sauthed!_TmVPtWyCByrJUs%HCJ5OjyPUH9UlJA5r%u1O2$nLQNg;Max-Age=3600;Expires=Fri, 14 Jul 2017 03:40:00 GMT;Path=/;HttpOnly;Secure"},
		{"c", "v", []func(*OutCookie) error{Path("/moo/")}, "c=v__!sauthed!_TmVPtWyCByrJUs%HCJ5OjyPUH9UlJA5r%u1O2$nLQNg;Path=/moo/;HttpOnly;Secure"},
		{"c", "v", []func(*OutCookie) error{Domain("fo-o2.com")}, "c=v__!sauthed!_TmVPtWyCByrJUs%HCJ5OjyPUH9UlJA5r%u1O2$nLQNg;Path=/;Domain=fo-o2.com;HttpOnly;Secure"},
		{"c", "v", []func(*OutCookie) error{Domain(".foo.com")}, "c=v__!sauthed!_TmVPtWyCByrJUs%HCJ5OjyPUH9UlJA5r%u1O2$nLQNg;Path=/;Domain=.foo.com;HttpOnly;Secure"},
		{"c", "v", []func(*OutCookie) error{ClientCanRead}, "c=v;Path=/;Secure"},
	}

	for _, test := range tests {
		cookie, err := NewOut(test.name, test.value, authenticator, test.opts...)
		if err != nil {
			t.Fatal("Failed to generate cookie '", test.result, "'")
		}
		rendered, err := cookie.Render()
		if err != nil {
			t.Fatal("Failed to render cookie '", test.result, "' with",
				err)
		}
		if test.result != rendered {
			t.Fatal("Failed to render cookie. Expected '" + string(test.result) + "', got '" + string(rendered) + "'")
		}

		var cookieIn string
		semicolonIdx := strings.Index(rendered, ";")
		if semicolonIdx == -1 {
			cookieIn = rendered
		} else {
			cookieIn = rendered[:semicolonIdx]
		}

		inCookies, rejected := ParseCookies(
			[]string{sessionCookie, cookieIn},
			&ConstantUnwrapper{authenticator},
		)
		if len(rejected) > 0 {
			t.Fatal("Ended up rejecting what should have been a good cookie:", cookieIn)
		}
		if inCookies.Count() != 2 {
			t.Fatal("Wrong number of parsed cookies for", cookieIn, inCookies.Count())
		}
	}
}

func TestBadAuthenticatorRendering(t *testing.T) {
	c, _ := NewOut("cookie", "value", BadAuthenticator{})
	_, err := c.Render()
	if err == nil {
		t.Fatal("Can sign cookies even when authenticator fails?")
	}
	c.Name()
}

func TestNonstandardOut(t *testing.T) {
	c, _ := NewNonstandardOut("cookie", "value is,", nil)
	val, _ := c.Render()
	if val != `cookie="value is,";Path=/;HttpOnly;Secure` {
		t.Fatal("nonstandard cookie didn't work", val)
	}
	_, err := NewNonstandardOut("cookie", "value;", nil)
	if err == nil {
		t.Fatal("Permitted to make illegal values in nonstandard cookie")
	}
}

type testIllegal struct {
	name  string
	value string
	opts  []func(*OutCookie) error
}

// This function verifies that illegal settings correctly fail.
func TestIllegalCookies(t *testing.T) {
	tests := []testIllegal{
		{" space", "v", []func(*OutCookie) error{}},
		{"\ttab", "v", []func(*OutCookie) error{}},
		{"n", " strict mode space", []func(*OutCookie) error{}},
		{"", "", []func(*OutCookie) error{}},
		{"", "\tmoo", []func(*OutCookie) error{}},
		{"n", "v", []func(*OutCookie) error{Duration(time.Duration(-2))}},
		{"n", "v", []func(*OutCookie) error{Duration(time.Millisecond)}},
		{"n", "v", []func(*OutCookie) error{Duration(time.Hour * 24 * 365 * 24)}},
		{"c", "\x10", []func(*OutCookie) error{}},
		{"c", "v", []func(*OutCookie) error{Path("/bad;path/")}},

		// lots of ways for domain to be illegal, aren't there?
		{"c", "v", []func(*OutCookie) error{Domain("")}},
		{"c", "v", []func(*OutCookie) error{Domain("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")}},
		{"c", "v", []func(*OutCookie) error{Domain("foo-.com")}},
		{"c", "v", []func(*OutCookie) error{Domain("foo..com")}},
		{"c", "v", []func(*OutCookie) error{Domain("foo.-com")}},
		{"c", "v", []func(*OutCookie) error{Domain("foo.com-")}},
		{"c", "v", []func(*OutCookie) error{Domain("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.com")}},
		{"c", "v", []func(*OutCookie) error{Domain("foo\t.com")}},
	}

	for idx, test := range tests {
		_, err := NewOut(test.name, test.value, nil, test.opts...)
		if err == nil {
			t.Fatal("Failed on illegal cookie", idx)
		}
	}
}

type ParseCookieTest struct {
	cookies  []string
	expected *InCookies
	rejected []string
}

func cookies(in ...*InCookie) *InCookies {
	inCookies := &InCookies{}
	for _, cookie := range in {
		inCookies.addInCookie(cookie)
	}
	return inCookies
}

func TestParseCookies(t *testing.T) {
	x, y := ParseCookies(nil, nil)
	if x.Count() > 0 || len(y) > 0 {
		t.Fatal("unexpected squeezed blood from a stone")
	}

	authenticator := secret.New([]byte("badsecret"))
	sessionID, _ := authenticator.Authenticate([]byte("session"), []byte("1"))
	sessionCookie := "session=" + string(sessionID)
	sessionID2, _ := authenticator.Authenticate([]byte("session"), []byte("2"))
	sessionCookie2 := "session=" + string(sessionID2)
	sessionIn := &InCookie{"session", "1", true}

	// simulate an old, timed-out authenticator or something
	oldAuthenticator := secret.New([]byte("oldsecret"))
	oldCookieVal, _ := oldAuthenticator.Authenticate([]byte("old"), []byte("old"))
	oldCookie := "old=" + string(oldCookieVal)
	oldSessionVal, _ := oldAuthenticator.Authenticate([]byte("session"), []byte("1"))
	oldSessionCookie := "session=" + string(oldSessionVal)

	for _, test := range []ParseCookieTest{
		{
			[]string{sessionCookie},
			cookies(sessionIn),
			[]string{},
		},

		{
			[]string{";" + sessionCookie + ";", "", ";", ";;"},
			cookies(sessionIn),
			[]string{},
		},

		{
			[]string{sessionCookie, "\x00=cow;x=\"y\"", "z=\x00"},
			cookies(&InCookie{"x", "y", false}, sessionIn),
			[]string{},
		},

		{
			[]string{"session=21", sessionCookie, "session=25"},
			cookies(sessionIn),
			[]string{},
		},

		{
			[]string{"session=21", "session=26", "session=25"},
			cookies(&InCookie{"session", "21", false}),
			[]string{},
		},

		{
			[]string{sessionCookie, sessionCookie2},
			cookies(sessionIn),
			[]string{},
		},

		{
			[]string{sessionCookie, oldCookie},
			cookies(sessionIn),
			[]string{"old"},
		},

		{
			[]string{oldCookie},
			cookies(),
			[]string{"old"},
		},

		{
			[]string{oldSessionCookie, oldCookie},
			cookies(),
			[]string{"session"},
		},
	} {
		cookies, rejected := ParseCookies(test.cookies, &ConstantUnwrapper{authenticator})
		if !reflect.DeepEqual(cookies, test.expected) ||
			!reflect.DeepEqual(rejected, rejected) {
			t.Fatal(fmt.Sprintf("Failed to correctly parse cookie \"%s\". Expected to get:\ncookies: %#v\nrejected: %#v\n\n but got instead: cookies: %#v\nrejected: %#v", test.cookies, test.expected, test.rejected, cookies, rejected))
		}
	}

	// test the case where the authentication just plain errors out
	c, rejected := ParseCookies([]string{sessionCookie}, NeverUnwrapper{})
	if !reflect.DeepEqual(c, cookies()) ||
		!reflect.DeepEqual(rejected, []string{"session"}) {
		t.Fatal("Did not correctly reject session when no auth found")
	}
}

func TestGettingFromInCookies(t *testing.T) {
	inCookies := cookies(
		&InCookie{"session", "1", true},
		&InCookie{"notsession", "2", false},
	)

	if inCookies.Get("moo") != nil {
		t.Fatal("Can spontaneously produce cookies")
	}
	if inCookies.Get("notsession") != nil {
		t.Fatal("can fetch unauthenticated cookies with just .Get")
	}
	if inCookies.Get("session").Name() != "session" {
		t.Fatal("Can't fetch authenticated cookies with .Get")
	}
	if inCookies.GetPossiblyUnauthenticated("session") == nil {
		t.Fatal("Can't fetch authenticated cookies with .GetPossiblyUnauthenticated")
	}
	if inCookies.GetPossiblyUnauthenticated("notsession") == nil {
		t.Fatal("Can't fetch unauthenticated cookies with .GetPossiblyUnauthenticated")
	}
}

// Tiny little testing mostly meant for coverage.
func TestCoverage(t *testing.T) {
	// test this doesn't crash, basically
	eci := errCookieInvalid{"cookie", "reason"}
	eci.Error()

	ic := &InCookie{"n", "v", true}
	ic.Name()
	ic.Value()
	ic.Authenticated()

	ics := cookies(&InCookie{"a", "b", true}, &InCookie{"c", "d", false})
	if ics.GoString() != "[InCookies: {a=b/authenticated=true, c=d/authenticated=false}]" {
		t.Fatal("InCookies GoString failing:", ics.GoString())
	}

	if isAuthed("v__!pauthed!_xw4yR0Ay22CRJFSbATXsYTGhd$GVa9TO1usooxD7iUE") {
		t.Fatal("something 1 wrong with isAuthed")
	}
	if isAuthed("v__!sauthed!_xw4yR0Ay22CRJFSbATXsYTGhd$G\x00a9TO1usooxD7iUE") {
		t.Fatal("something 2 wrong with isAuthed")
	}

	// Note this is the signature above with the E on the end cut off
	if maybeSignature("xw4yR0Ay22CRJFSbATXsYTGhd$GVa9TO1usooxD7iU") {
		t.Fatal("maybeSignature misses length")
	}
	if maybeSignature("xw4yR0Ay22CRJFSbATXsYTGhd$GVa9TO1usooxD7iU\x00") {
		t.Fatal("maybeSignature misses length")
	}
}

type ConstantUnwrapper struct {
	au secret.AuthenticationUnwrapper
}

func (cu *ConstantUnwrapper) GetAuthenticationUnwrapper(string) (secret.AuthenticationUnwrapper, error) {
	return cu.au, nil
}

type NeverUnwrapper struct{}

func (nu NeverUnwrapper) GetAuthenticationUnwrapper(string) (secret.AuthenticationUnwrapper, error) {
	return nil, errors.New("I never have anything")
}

type BadAuthenticator struct{}

func (ba BadAuthenticator) Authenticate(...[]byte) ([]byte, error) {
	return nil, errors.New("I am too feeble to authenticate!")
}

func ExampleNewOut() {
	// Create an outgoing cookie that is forever insecure, unauthenticated,
	// and unknowable by the client. Truly...
	cookie, _ := NewOut(
		"this_cookie_is_so_emo",
		"true",
		nil,
		Forever, Insecure,
		Domain("despair.com"),
	)

	val, _ := cookie.Render()
	fmt.Println(val)

	// Output: this_cookie_is_so_emo=true;Expires=Tue, 19 Jan 2038 03:14:07 GMT;Path=/;Domain=despair.com;HttpOnly
}
