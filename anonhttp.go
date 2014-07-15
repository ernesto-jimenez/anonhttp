package anonhttp

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	// DefaultProxyQueue is initialized with the DefaultProxyQueue
	DefaultProxyQueue = NewProxyQueue()
)

// AddProxy adds a proxy to the pool of available proxies
func AddProxy(u *url.URL) {
	DefaultProxyQueue.Enqueue(u)
}

// SetFetcher sets the fetcher for DefaultProxyQueue
func SetFetcher(fetcher ProxyFetcher) {
	DefaultProxyQueue.Fetcher = fetcher
}

// Transport is a http.RoundTripper that proxies requests through a proxy from the pool of available proxies
type Transport struct {
	Requests   int
	ProxyQueue *ProxyQueue
	timeout    time.Duration
}

// NewTransport returns a Transport
func NewTransport(requests int, timeout time.Duration) http.RoundTripper {
	return &Transport{
		Requests:   requests,
		ProxyQueue: DefaultProxyQueue,
		timeout:    timeout,
	}
}

type requestResult struct {
	res *http.Response
	err error
}

type requestError struct {
	errs []error
}

func (err *requestError) Error() string {
	msgs := make([]string, len(err.errs)+1)
	msgs[0] = "no http request succeded. Errors:"
	for i, err := range err.errs {
		msgs[i+1] = " * " + err.Error()
	}
	return strings.Join(msgs, "\n")
}

func (t *Transport) dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, t.timeout)
}

// RoundTrip proxies request through one or many proxies from the available pool
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	resCh := make(chan requestResult)
	finished := make(chan struct{})
	request := func() {
		proxy, err := t.ProxyQueue.Get()
		if err != nil {
			select {
			case resCh <- requestResult{nil, err}:
			case <-finished:
			}
			return
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxy),
			Dial:  t.dialTimeout,
		}
		res, err := transport.RoundTrip(req)
		if err == nil && res.StatusCode != 200 {
			err = fmt.Errorf("invalid response status code: %d", res.StatusCode)
			fmt.Printf("invalid response status code: %d\n", res.StatusCode)
		} else {
			if err == nil {
				t.ProxyQueue.Enqueue(proxy)
			} else {
				fmt.Printf("failed response: %s\n", err.Error())
			}
		}
		select {
		case resCh <- requestResult{res, err}:
		case <-finished:
			return
		}
	}
	for i := 0; i < t.Requests; i++ {
		go request()
	}
	err := &requestError{}
	for i := 0; i < t.Requests; i++ {
		res := <-resCh
		if res.err == nil && res.res.StatusCode == 200 {
			fmt.Printf("ok response status code: %d\n", res.res.StatusCode)
			close(finished)
			return res.res, nil
		}
		err.errs = append(err.errs, res.err)
	}

	return nil, err
}
