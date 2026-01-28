package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

type DebugTransport struct{}

func (DebugTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	b, err := httputil.DumpRequestOut(r, false)
	if err != nil {
		return nil, err
	}
	log.Println(string(b))
	return http.DefaultTransport.RoundTrip(r)
}

func reverseproxy() *httputil.ReverseProxy {
	rewrite := func(req *httputil.ProxyRequest) {
		req.SetXForwarded() // set headers on Out request

		destUrl, _ := url.Parse(MainDestination.url)
		log.Println("Sending to main:", destUrl.String())
		setRequestPath(req.Out, destUrl)
	}

	proxy := &httputil.ReverseProxy{Rewrite: rewrite}
	proxy.Transport = DebugTransport{}

	return proxy
}

// handleNotification processes the notification and returns whether to send to main
func handleNotification(req *http.Request) bool {
	// Read body
	body, _ := io.ReadAll(req.Body)
	log.Println(string(body))

	// Reset body for potential proxy use
	req.Body = io.NopCloser(bytes.NewBuffer(body))

	dests := destinations(string(body))

	// If no destinations matched, send to main only
	if len(dests) == 0 {
		log.Println("No matching destinations, sending to main only")
		return true
	}

	// Check if any destination wants to also send to main
	shouldSendToMain := false
	for _, dest := range dests {
		if dest.sendToMain {
			shouldSendToMain = true
			break
		}
	}

	// Mirror to all matching destinations
	for _, dest := range dests {
		log.Println("Mirroring to", dest.url)

		// Clone request for the mirror
		mReq := req.Clone(req.Context())
		mReq.Body = io.NopCloser(bytes.NewReader(body))

		mirrorRequest(*mReq, dest.url)
	}

	// Reset body again for potential proxy use
	req.Body = io.NopCloser(bytes.NewBuffer(body))

	return shouldSendToMain
}

var authKey string

func main() {
	log.Println("Starting")

	// Load .env file (ignore error if file doesn't exist)
	if err := godotenv.Load(); err == nil {
		log.Println("Loaded .env file")
	}

	// Load configurations & validate
	loadDestinations()
	authKey = os.Getenv("AUTH_KEY")
	if authKey == "" {
		log.Fatalln("Missing AUTH_KEY environment variable")
	}
	log.Println("Auth key loaded")

	proxy := reverseproxy()

	// Http server
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		log.Printf("Request: %s %s from %s", req.Method, req.URL.Path, req.RemoteAddr)

		// Redirect GET root to github repo
		if req.Method == "GET" && req.RequestURI == "/" {
			log.Printf("Redirecting to GitHub repo")
			http.Redirect(w, req, "https://github.com/hackclub/coolify-slack-conductor", 302)
			return
		}

		// Having authentication here prevents ppl from spamming our slack channels
		if len(req.URL.Query()["key"]) == 0 || req.URL.Query()["key"][0] != authKey {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			log.Printf("Auth failed: %s %s from %s", req.Method, req.URL.Path, req.RemoteAddr)
			return
		}

		log.Printf("Auth successful: %s %s from %s", req.Method, req.URL.Path, req.RemoteAddr)

		// Process notification and determine if we should send to main
		shouldSendToMain := handleNotification(req)

		if shouldSendToMain {
			// Pass request to the reverse proxy (sends to main)
			proxy.ServeHTTP(w, req)
		} else {
			// Don't send to main, just return OK
			log.Println("Skipping main destination (no send-to-main flag set)")
			w.WriteHeader(http.StatusOK)
		}
	})

	log.Println("Server listening on :8080")
	log.Fatalln(http.ListenAndServe(":8080", nil))
}
