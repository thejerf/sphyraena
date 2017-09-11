package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thejerf/sphyraena/context"
	"github.com/thejerf/sphyraena/sphyrw"
)

func ptr(i int) *int {
	return &i
}

func sf1(*sphyrw.SphyraenaResponseWriter, *context.Context) {}
func sf2(*sphyrw.SphyraenaResponseWriter, *context.Context) {}

var SF1 = context.HandlerFunc(sf1)
var SF2 = context.HandlerFunc(sf2)

func (sr *SphyraenaRouter) mustGet(t *testing.T, url string) context.Handler {
	req, _ := http.NewRequest("GET", url, nil)
	ctx, _ := sr.sphyraenaState.NewContext(httptest.NewRecorder(), req)
	handler, _, err := sr.getStrest(ctx)
	if err != nil {
		t.Fatal("Could not get request:", err)
	}
	return handler
}

// yup, this is cheating. but then, note the final five characters of this
// file's name, before the ".go"...
func samefunc(a, b context.Handler) bool {
	return fmt.Sprintf("%#v", a) == fmt.Sprintf("%#v", b)
}

func TestMinimalFunctionality(t *testing.T) {
	sr := New(context.NewSphyraenaState(nil, nil))

	if !samefunc(SF1, SF1) {
		t.Fatal("samefunc says the same function is different")
	}
	if samefunc(SF1, SF2) {
		t.Fatal("same func says the different functions are the same")
	}

	sr.AddLocationReturn("/home/product/firmware", SF1)

	req, _ := http.NewRequest("GET", "http://jerf.org/home/product/firmware", nil)
	ctx, _ := sr.sphyraenaState.NewContext(httptest.NewRecorder(), req)
	rreq := newRequest(ctx)
	result := sr.Route(rreq)
	if result.Error != nil {
		t.Fatal(result.Error)
	}

	// Verify the static location must exactly match... oh, how many hours
	// I've lost on failing to check this sort of thing...
	req2, _ := http.NewRequest("GET", "http://jerf.org/home/product/firmwares", nil)
	ctx2, _ := sr.sphyraenaState.NewContext(httptest.NewRecorder(), req2)
	rreq2 := newRequest(ctx2)
	result = sr.Route(rreq2)
	if result.Handler != nil {
		t.Fatal("StaticLocation doesn't require a full path match")
	}

	if !samefunc(sr.mustGet(t, "http://jerf.org/home/product/firmware"), SF1) {
		fmt.Printf("Got: %#v 1: %#v 2: %#v\n", sr.mustGet(t, "http://jerf.org/home/product/firmware"), SF1, SF2)
		t.Fatal("Wrong route returned (first)")
	}

	sr.AddLocationReturn("/home/production/yes", SF2)
	if !samefunc(sr.mustGet(t, "http://jerf.org/home/product/firmware"), SF1) {
		t.Fatal("adding second route failed the original route")
	}
	if !samefunc(sr.mustGet(t, "http://jerf.org/home/production/yes"), SF2) {
		t.Fatal("Second route doesn't work")
	}
}

func TestTreeOfLocations(t *testing.T) {
	sr := New(context.NewSphyraenaState(nil, nil))
	l1 := sr.Location("/test1")
	l1.AddLocationReturn("/test2", SF2)

	req, _ := http.NewRequest("GET", "http://jerf.org/test1/test2", nil)
	ctx, _ := sr.sphyraenaState.NewContext(httptest.NewRecorder(), req)
	rreq := newRequest(ctx)
	result := sr.Route(rreq)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	if !samefunc(result.Handler, SF2) {
		t.Fatal("Nested locations didn't work")
	}
}

func TestLocationForward(t *testing.T) {
	sr := New(context.NewSphyraenaState(nil, nil))
	sr.AddLocationForward("/", SF1)

	req, _ := http.NewRequest("GET", "http://jerf.org/anything/goes", nil)
	ctx, _ := sr.sphyraenaState.NewContext(httptest.NewRecorder(), req)
	rreq := newRequest(ctx)
	result := sr.Route(rreq)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	if !samefunc(result.Handler, SF1) {
		t.Fatal("Location didn't match correctly")
	}
}
