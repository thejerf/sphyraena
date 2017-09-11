package unicode

import "golang.org/x/text/unicode/norm"

// An NFKCNormalized value is a value that has been run through the NFKC
// Unicode normalization process. This is a somewhat-destructive procedure
// for normalizing things designed to inhibit attempts to deceive.
//
// See http://www.unicode.org/reports/tr36/ for some background.
type NFKCNormalized struct {
	value string
}

// String returns the string value of the normalized value, implementing
// fmt.Stringer in the process.
func (nn *NFKCNormalized) String() string {
	return nn.value
}

// GoString implements fmt.GoStringer, returning the string value of the string.
func (nn *NFKCNormalized) GoString() string {
	return nn.value
}

func NFKCNormalize(s string) NFKCNormalized {
	return NFKCNormalized{norm.NFKC.String(s)}
}
