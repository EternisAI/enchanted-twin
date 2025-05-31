package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {
	target, _ := url.Parse("https://openrouter.ai")
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Use Director instead of ModifyRequest
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("User-Agent", "MyApp/1.0")
		req.Header.Set("X-Forwarded-For", "")
		req.Header.Del("X-Forwarded-Host")
		req.Host = target.Host
	}

	log.Fatal(http.ListenAndServe(":12000", proxy))
}
