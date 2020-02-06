package secret

import (
	"errors"
	"testing"
	"time"
)

func TestGenerator(t *testing.T) {
	g := NewGenerator(0)

	go g.Serve()
	defer g.Stop()

	s := g.Get()
	if s == nil {
		t.Fatal("Can't get a secret from the generator.")
	}
}

func TestGeneratorErrorHandleng(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Failed to correctly error out with badReader")
		}
	}()
	g := NewGenerator(0)
	g.randReader = badReader{}
	go func() {
		<-g.output
	}()
	g.Serve()
}

func TestGeneatorBadReadHandling(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Failed to correctly handle error with slow reader")
		}
	}()
	g := NewGenerator(0)
	g.randReader = slowReader{}
	go func() {
		<-g.output
	}()
	g.Serve()
}

func BenchmarkSecretGeneration(b *testing.B) {
	for n := 0; n < b.N; n++ {
		Get()
	}
}

func BenchmarkSecretServing(b *testing.B) {
	g := NewGenerator(b.N)
	go g.Serve()
	defer g.Stop()

	// wait for it to fill up
	for len(g.output) < b.N {
		time.Sleep(time.Millisecond)
	}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		g.Get()
	}
}

type badReader struct{}

func (br badReader) Read([]byte) (int, error) {
	return 0, errors.New("aaaaaaaa")
}

type slowReader struct{}

func (sr slowReader) Read(b []byte) (int, error) {
	b = append(b, 'h', 'i')
	return 2, nil
}
