package anonhttp

import (
	"testing"
	"time"

	"net/http"
	"net/http/httptest"
	"net/url"
)

func TestAddProxy(t *testing.T) {
	want := "http://localhost:8080"
	proxy, _ := url.Parse(want)
	AddProxy(proxy)
	got, err := DefaultProxyQueue.Get()
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestAnonTransport(t *testing.T) {
	ts, proxy, ch := prepareProxy()
	defer ts.Close()
	defer proxy.Close()

	pu, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	AddProxy(pu)
	AddProxy(pu)
	AddProxy(pu)
	c := &http.Client{Transport: NewTransport(3, 5*time.Second)}
	_, err = c.Head(ts.URL)
	if err != nil {
		t.Error(err)
	}
	got := <-ch
	want := "proxy for " + ts.URL + "/"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func prepareProxy() (ts *httptest.Server, proxy *httptest.Server, ch chan string) {
	ch = make(chan string, 1)
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case ch <- "real server":
		default:
		}
	}))
	proxy = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case ch <- "proxy for " + r.URL.String():
		default:
		}
	}))
	return
}
