package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

type LoadBalancer struct {
	port            string
	roundRobinCount int
	servers         []Server
}

func NewLoadBalancer(port string, servers []Server) *LoadBalancer {
	return &LoadBalancer{
		port:            port,
		roundRobinCount: 0,
		servers:         servers,
	}
}

// getNextServAddr returns the address of the next available server to send a request to using simple round-robin algo
func (lb *LoadBalancer) getNextAvailableServer() Server {
	server := lb.servers[lb.roundRobinCount%len(lb.servers)]
	for !server.IsAlive() {
		lb.roundRobinCount++
		server = lb.servers[lb.roundRobinCount%len(lb.servers)]
	}
	lb.roundRobinCount++

	return server
}

func (lb *LoadBalancer) serveProxy(rw http.ResponseWriter, req *http.Request) {
	targetServer := lb.getNextAvailableServer()

	// logging
	fmt.Printf("forwarding request to address %q\n", targetServer.Address())

	// could delete pre-existing X-Forwarded-For header to prever IP spoofing
	targetServer.Serve(rw, req)
}

type Server interface {
	// Address returns the address with which to access the server
	Address() string

	// IsAlive returns true if the server is alive and is able to server requests
	IsAlive() bool

	// Serve uses this server to process the request
	Serve(rw http.ResponseWriter, req *http.Request)
}

type simpleServer struct {
	addr  string
	proxy *httputil.ReverseProxy
}

func (s *simpleServer) Address() string {
	return s.addr
}

func (s *simpleServer) IsAlive() bool {
	return true
}

func (s *simpleServer) Serve(rw http.ResponseWriter, req *http.Request) {
	s.proxy.ServeHTTP(rw, req)
}

// this creates a new server based on it's URL returns an object of that type with it's new reverse proxy address
func newSimpleServer(addr string) *simpleServer {
	serverUrl, err := url.Parse(addr)
	handleErr(err)

	return &simpleServer{
		addr:  addr,
		proxy: httputil.NewSingleHostReverseProxy(serverUrl),
	}
}

// if there is an error, just bail out! *not for production use*
func handleErr(err error) {
	if err != nil {
		fmt.Printf("error : %v\n", err)
		os.Exit(1)
	}
}

func main() {
	servers := []Server{
		newSimpleServer("https://www.google.com"),
		newSimpleServer("https://www.bing.com"),
		newSimpleServer("https://www.duckduckgo.com"),
	}

	lb := NewLoadBalancer("8000", servers)
	handleRedirect := func(rw http.ResponseWriter, req *http.Request) {
		lb.serveProxy(rw, req)
	}

	// register a proxy handler to handle all requests
	http.HandleFunc("/", handleRedirect)

	fmt.Printf("serving requests at 'localhost:%s'\n", lb.port)
	http.ListenAndServe(":"+lb.port, nil)
}
