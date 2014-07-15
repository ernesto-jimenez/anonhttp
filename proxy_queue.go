package anonhttp

import (
	"errors"
	"net/url"
)

// ProxyQueue is used to store a queue of available proxies to be used
type ProxyQueue struct {
	errorLimit int

	// Channels used to modify the proxy queue from consuming goroutines
	getter chan chan *url.URL
	setter chan []*url.URL

	// proxies holds a queue of all available proxies
	proxies []*url.URL
	// errorCount counts the amount of errors for each proxy in proxies
	// when a proxy reaches the limit from errorLimit, it is removed from the queue
	errorCount map[string]int

	// Channel used close proxy list
	closing chan chan struct{}

	Fetcher ProxyFetcher
}

// NewProxyQueue initializes a ProxyQueue
func NewProxyQueue() *ProxyQueue {
	q := &ProxyQueue{
		errorLimit: -1,
		getter:     make(chan chan *url.URL),
		setter:     make(chan []*url.URL),
		errorCount: make(map[string]int),
		closing:    make(chan chan struct{}),
		Fetcher: func() []*url.URL {
			panic("no fetcher set")
		},
	}
	go q.loop()
	return q
}

// ProxyFetcher fetches a new set of proxies
type ProxyFetcher func() []*url.URL

func (q *ProxyQueue) addProxy(proxy *url.URL) {
	u := proxy.String()
	// Avoid adding duplicates
	if q.errorCount[u] > 0 {
		return
	}
	q.proxies = append(q.proxies, proxy)
	q.errorCount[u] = q.errorLimit
}

func (q *ProxyQueue) addProxies(proxies []*url.URL) {
	for _, proxy := range proxies {
		q.addProxy(proxy)
	}
}

//func (q *ProxyQueue) enqueueWithError(proxy *url.URL) {
//url := proxy.String()
//if q.errorCount[url] == 0 {
//panic("tried to increase error count for proxy that wasn't in the queue")
//}
//q.errorCount[url]--
//if q.errorCount[url] == 0 {
//return
//}
//q.proxies = append(q.proxies, proxy)
//}

func (q *ProxyQueue) fetch() chan []*url.URL {
	fetching := make(chan []*url.URL)
	go func() {
		fetching <- q.Fetcher()
		close(fetching)
	}()
	return fetching
}

func (q *ProxyQueue) loop() {
	var requests []chan *url.URL
	var fetching chan []*url.URL
	for {
		var nextRequest chan *url.URL
		var response *url.URL
		if len(requests) > 0 && len(q.proxies) > 0 {
			nextRequest = requests[0]
			response = q.proxies[0]
		}
		select {
		case proxies := <-fetching:
			q.addProxies(proxies)
			fetching = nil
		case newProxies := <-q.setter:
			q.addProxies(newProxies)
		case request := <-q.getter:
			if len(q.errorCount) == 0 && fetching == nil {
				fetching = q.fetch()
			}
			requests = append(requests, request)
		case nextRequest <- response:
			requests = requests[1:]
			q.proxies = q.proxies[1:]
		case ok := <-q.closing:
			for _, request := range requests {
				close(request)
			}
			close(q.setter)
			close(q.getter)
			close(ok)
			return
		}
	}
}

// Close stops the queue and cleans up
func (q *ProxyQueue) Close() {
	closed := make(chan struct{})
	q.closing <- closed
	<-closed
}

// Initialize sets the list of proxies to be used in the queue
func (q *ProxyQueue) Initialize(proxies []*url.URL) {
	q.setter <- proxies
}

// Enqueue adds a proxy to the queue
func (q *ProxyQueue) Enqueue(proxy *url.URL) {
	q.setter <- []*url.URL{proxy}
}

// Get returns the next proxy in the queue
func (q *ProxyQueue) Get() (*url.URL, error) {
	request := make(chan *url.URL)
	q.getter <- request
	proxy, ok := <-request
	if !ok {
		return nil, errors.New("proxy queue closed")
	}
	return proxy, nil
}
