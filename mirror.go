package main

import (
	"log"
	"net/http"
	"net/url"
)

func setRequestPath(req *http.Request, target *url.URL) {
	req.URL = target
	req.Host = ""
}

func mirrorRequest(req http.Request, destUrl string) {
	target, _ := url.Parse(destUrl)
	setRequestPath(&req, target)

	req.RequestURI = "" // Can not be set for client requests

	resp, err := http.DefaultClient.Do(&req)
	if err != nil {
		log.Printf("Mirror request failed to %s: %v", destUrl, err)
	} else {
		log.Printf("Mirror request to %s: %d", destUrl, resp.StatusCode)
		resp.Body.Close()
	}
}
