package secret

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
)

// sssshhh... this is the SECRET test. don't tell anyone about me!

var badwriterErr = errors.New("bad writer")

type badwriter struct{}

func (bw badwriter) Write(b []byte) (int, error) {
	return 0, badwriterErr
}

type escapeTest struct {
	in  string
	out string
}

func TestEscapedWrite(t *testing.T) {
	t.Parallel()
	for _, test := range []escapeTest{
		{"a", "a"},
		{"a\x00a", "a\x00\x00a"},
		{"", ""},
	} {
		var buf bytes.Buffer
		x, err := EscapedWrite(&buf, []byte(test.in))
		if err != nil {
			t.Fatal("Error:", err)
		}
		if x != len(test.out) {
			t.Fatal("Mismatch between lengths")
		}
		if string(buf.Bytes()) != test.out {
			t.Fatal("Mismatched output")
		}
	}

	x, err := EscapedWrite(badwriter{}, []byte("moo"))
	if x != 0 || err != badwriterErr {
		t.Fatal("EscapedWrite fails to handle writer errors")
	}
	x, err = EscapedWrite(badwriter{}, []byte("m\x00oo"))
	if x != 0 || err != badwriterErr {
		t.Fatal("EscapedWrite fails to handle writer errors")
	}
}

func TestSecretErrorCaseCoverage(t *testing.T) {
	t.Parallel()

	sEmpty := New([]byte(""))
	_, err := sEmpty.Authenticate([]byte("moo"))
	if err != ErrNoSecretKey {
		t.Fatal("can authenticate without a key")
	}
	_, err = sEmpty.UnwrapAuthentication([]byte("moo"))
	if err != ErrNoSecretKey {
		t.Fatal("can unwrap auth without a key")
	}

	var sNil *Secret
	_, err = sNil.Authenticate([]byte("moo"))
	if err != ErrNoSecretKey {
		t.Fatal("can authenticate with a nil *Secret")
	}
	_, err = sNil.UnwrapAuthentication([]byte("moo"))
	if err != ErrNoSecretKey {
		t.Fatal("can unwrap auth with a nil *Secret")
	}

	s := New([]byte("secret"))
	_, err = s.UnwrapAuthentication([]byte("moo"))
	if err != ErrNotAuthenticated {
		t.Fatal("Can authenticate something with no sig")
	}
}

func TestSecrets(t *testing.T) {
	t.Parallel()

	tests := [][][]byte{
		{[]byte("ab\x00")},
		{[]byte("a")},
		{[]byte("ab\x00cd")},
		{[]byte("\x00abcd")},
		{[]byte("\x00")},
		{[]byte("abc"), []byte("def")},
		{[]byte("ab\x00"), []byte("\x00\x00")},
		{[]byte("abc"), []byte("")},
	}
	s := New([]byte("secret"))
	wrong := New([]byte("totally public"))

	for _, testByteSlice := range tests {
		authedBytes, err := s.Authenticate(testByteSlice...)
		if err != nil {
			t.Fatal("Error authenticating bytes:", testByteSlice)
		}

		var newSlice [][]byte
		newSlice = append(newSlice, testByteSlice[:len(testByteSlice)-1]...)
		newSlice = append(newSlice, authedBytes)

		checked, err := s.UnwrapAuthentication(newSlice...)
		if err != nil {
			t.Fatal("Error unauthenticating bytes:", testByteSlice)
		}
		if string(checked) != string(testByteSlice[len(testByteSlice)-1]) {
			t.Fatal("Authed bytes did not come back cleanly:",
				string(checked),
				string(testByteSlice[len(testByteSlice)-1]))
		}

		_, err = wrong.UnwrapAuthentication(newSlice...)
		if err == nil {
			t.Fatal("The wrong secret was able to unwrap the auth!")
		}
	}

	authedLeft, _ := s.Authenticate(
		[]byte("A\x00\x01"), []byte("B"), []byte("C"))
	authedRight, _ := s.Authenticate(
		[]byte("A"), []byte("\x00\x01B"), []byte("C"))
	if reflect.DeepEqual(authedLeft, authedRight) {
		// there's probably a technical name for this attack but I don't
		// know what it is
		t.Fatal("authentication does not protect correctly against delimiters")
	}
}

func TestMarshaling(t *testing.T) {
	s := New([]byte("secret"))

	// by inspection, can not error
	bin, _ := s.MarshalBinary()
	s2 := Secret{}

	// by inspection, can not error
	_ = s2.UnmarshalBinary(bin)

	if !reflect.DeepEqual(s, &s2) {
		t.Fatal("MarshalBinary -> UnmarshalBinary not identity")
	}

	s3 := Secret{}
	text, _ := s.MarshalText()
	_ = s3.UnmarshalText(text)
	if !reflect.DeepEqual(s, &s3) {
		t.Fatal("MarshalText -> UnmarshalText not identity")
	}

	s4 := Secret{}
	err := s4.UnmarshalText([]byte("X"))
	if err == nil {
		t.Fatal("Can unmarshal things not hex-encoded")
	}
}
