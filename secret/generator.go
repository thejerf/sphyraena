/*

Package secret manages the types and generation of secrets.

Secrets are chunks of random bytes that can be used for authenticating that
some bytes were signed by this given secret.

*/
package secret

import (
	"crypto/rand"
	"fmt"
	"io"
)

// A SessionSecretGenerator provides SessionSecrets. These are cheaper to
// obtain that session keys because there's no HMAC'ing, but this can still
// be expensive.
type Generator struct {
	output     chan *Secret
	stop       chan struct{}
	randReader io.Reader
}

// NewGenerator returns an object from which secrets can be extracted. It
// must have .Serve() called on it in a goroutine before it will serve any
// secrets.
//
// The bufferSize is the number of secrets to pregenerate before any are
// requested.
//
// (Note that the buffering is to improve latency for requests, by getting
// the potentially-expensive generation step out of the critical path for a
// page. Do not expect this to increase throughput. It does not generally
// need to be set very large.)
func NewGenerator(bufferSize int) *Generator {
	if bufferSize == 0 {
		bufferSize = 128
	}

	return &Generator{
		make(chan *Secret, bufferSize),
		make(chan struct{}),
		rand.Reader,
	}
}

// Serve implements the Service interface from Suture.
func (g *Generator) Serve() {
	for {
		select {
		case g.output <- g.generate():
		case <-g.stop:
			return
		}
	}
}

// Stop implements the Service interface from Suture.
func (g *Generator) Stop() {
	g.stop <- struct{}{}
}

// .Get returns an Secret. Threadsafe.
func (g *Generator) Get() *Secret {
	return <-g.output
}

func (g *Generator) generate() *Secret {
	b := make([]byte, 32)
	n, err := g.randReader.Read(b)

	// these are purely internal panics, handled by suture
	if err != nil {
		panic(fmt.Errorf("While making secret keys, couldn't read from PSRNG: %s", err.Error()))
	}
	if n != 32 {
		panic(fmt.Errorf("While making secret keys, could only read %d bytes", n))
	}
	return &Secret{b}
}

// Get retrieves a new secret from the cryptographically-random number
// generator, synchronously. This is especially useful for testing.
func Get() *Secret {
	b := make([]byte, 32)
	n, err := rand.Reader.Read(b)

	if err != nil {
		panic(fmt.Errorf("While making secret keys, couldn't read PSRNG: %s", err.Error()))
	}
	if n != 32 {
		panic(fmt.Errorf("While making secret keys, could only read %d bytes", n))
	}
	return &Secret{b}
}

type Server interface {
	Get() *Secret
}

type directSecretServer struct{}

func (dss directSecretServer) Get() *Secret {
	return Get()
}

// DirectSecretServer serves secrets out directly
var DirectSecretServer = directSecretServer{}
