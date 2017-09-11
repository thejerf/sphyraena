package session

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"
)

// we use this as the quote-unquote "random" reader for testing.
// as casual inspection will reveal, this is CLEARLY
// cryptographically-secure grade randomness.
func newConstantBytesBuffer() io.Reader {
	return bytes.NewBuffer([]byte(`012345678901234567890
    012345678901234567890123456789012345678901234567890
    012345678901234567890123456789012345678901234567890
    012345678901234567890123456789012345678901234567890
    012345678901234567890123456789012345678901234567890
    012345678901234567890123456789012345678901234567890
    012345678901234567890123456789012345678901234567890
    012345678901234567890123456789012345678901234567890
    012345678901234567890123456789012345678901234567890
    012345678901234567890123456789012345678901234567890
    012345678901234567890123456789012345678901234567890
    012345678901234567890123456789012345678901234567890`))
}

func panics(f func()) (panics bool) {
	defer func() {
		if r := recover(); r != nil {
			panics = true
		}
	}()

	f()
	return
}

func TestSessionGeneration(t *testing.T) {
	skg := NewSessionIDGenerator(0, []byte("0123456789012345"))
	skg.randReader = newConstantBytesBuffer()

	b := make([]byte, 32)
	sessionKey := skg.generate(b)
	if sessionKey != "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwCiAgICAwMTIzNDWz4U5AH8jUMoBhAtbJ+gOiH82JSrDZAviWMgGR2d5nGg==" {
		t.Fatal("Unexpected session key generated:", sessionKey)
	}
	if !skg.Check(sessionKey) {
		t.Fatal("Session key validation failed")
	}

	// first byte is changed to be lowercase
	if skg.Check("mDEyMzQ1Njc4OTAxMjM0NTY3ODkwCiAgICAwMTIzNDWz4U5AH8jUMoBhAtbJ+gOiH82JSrDZAviWMgGR2d5nGg==") {
		t.Fatal("Session keys validate even when invalid")
	}
}

// this function is to be read in conjunction with coverage testing, to
// ensure that the relevant branches of Check are being exercised.
func TestInvalidSessions(t *testing.T) {
	skg := NewSessionIDGenerator(0, []byte("0123456789012345"))
	skg.randReader = newConstantBytesBuffer()

	if skg.Check("moo") {
		t.Fatal("overly short sessions pass")
	}

	if skg.Check("!!!!MzQ1Njc4OTAxMjM0NTY3ODkwCiAgICAwMTIzNDWz4U5AH8jUMoBhAtbJ+gOiH82JSrDZAviWMgGR2d5nGg==") {
		t.Fatal("non-base64 values accepted")
	}
}

// in theory, this is so corner case it can't happen with a functioning Go
// install. In practice... who knows?
func TestInsufficientCrypoReaders(t *testing.T) {
	tooSmall := bytes.NewBuffer([]byte("abc"))
	skg := NewSessionIDGenerator(0, nil)
	skg.randReader = tooSmall

	sessionKey := make([]byte, 32, 64)
	if !panics(func() {
		skg.generate(sessionKey)
	}) {
		t.Fatal("Can still make keys even when the random generator isn't big enough")
	}

	skg = NewSessionIDGenerator(0, nil)
	skg.randReader = BadReader{}
	if !panics(func() {
		skg.generate(sessionKey)
	}) {
		t.Fatal("Can still make session keys even when the reader fails")
	}
}

func TestDefault(t *testing.T) {
	skg := NewSessionIDGenerator(0, nil)
	if len(skg.hmacKey) != 32 {
		t.Fatal("didn't correctly populate on empty key")
	}
}

func TestServeInterface(t *testing.T) {
	skg := NewSessionIDGenerator(0, nil)
	go skg.Serve()

	skg.Get()
	skg.Stop()

	// can't really test the Serve function is properly terminated in any
	// safe way here
}

type BadReader struct{}

func (br BadReader) Read(p []byte) (int, error) {
	return 0, io.EOF
}

func BenchmarkGetting32Bytes(b *testing.B) {
	s := make([]byte, 32)
	for i := 0; i < b.N; i++ {
		n, err := rand.Read(s)
		if err != nil || n != 32 {
			panic("getting 32 bytes failed")
		}
	}
}
