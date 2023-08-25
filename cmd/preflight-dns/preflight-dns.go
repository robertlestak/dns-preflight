package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/robertlestak/preflight-dns/pkg/preflightdns"
	log "github.com/sirupsen/logrus"
)

func server(addr string) error {
	l := log.WithFields(log.Fields{
		"fn": "server",
	})
	l.Debug("starting server")
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		pf := &preflightdns.PreflightDNS{}
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(pf); err != nil {
			l.WithError(err).Error("error decoding request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := pf.Run(); err != nil {
			l.WithError(err).Error("error running preflight")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	return http.ListenAndServe(addr, nil)
}

func init() {
	ll, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		ll = log.InfoLevel
	}
	log.SetLevel(ll)
}

func main() {
	l := log.WithFields(log.Fields{
		"app": "preflight-dns",
	})
	l.Debug("starting preflight-dns")
	preflightFlags := flag.NewFlagSet("preflight-dns", flag.ExitOnError)
	endpoint := preflightFlags.String("endpoint", "", "endpoint to check")
	logLevel := preflightFlags.String("log-level", log.GetLevel().String(), "log level")
	method := preflightFlags.String("method", "GET", "method to use")
	body := preflightFlags.String("body", "", "body to send")
	headers := preflightFlags.String("headers", "", "headers to send. comma separated list of key=value")
	new := preflightFlags.String("new", "", "new hostname/ip to use")
	serverMode := preflightFlags.Bool("server", false, "run in server mode")
	serverAddr := preflightFlags.String("server-addr", ":8080", "server address to listen on")
	timeout := preflightFlags.Duration("timeout", 5*time.Second, "timeout for requests")
	lib := preflightFlags.Bool("lib", false, "lower is better. default is exact status code match.")
	configFile := preflightFlags.String("config", "", "config file to use")
	equiv := preflightFlags.Bool("equiv", false, "print sh equivalent command")
	preflightFlags.Parse(os.Args[1:])
	ll, err := log.ParseLevel(*logLevel)
	if err != nil {
		ll = log.InfoLevel
	}
	log.SetLevel(ll)
	preflightdns.Logger = l.Logger
	pf := &preflightdns.PreflightDNS{
		Endpoint:      *endpoint,
		Method:        *method,
		Body:          *body,
		New:           *new,
		LowerIsBetter: *lib,
		Timeout:       *timeout,
		Equiv:         *equiv,
	}
	if *headers != "" {
		pf.Headers = make(map[string]string)
		for _, h := range strings.Split(*headers, ",") {
			parts := strings.Split(h, "=")
			pf.Headers[parts[0]] = parts[1]
		}
	}
	if *configFile != "" {
		if pf, err = preflightdns.LoadConfig(*configFile); err != nil {
			l.WithError(err).Error("error loading config")
			os.Exit(1)
		}
	}
	if *serverMode {
		if err := server(*serverAddr); err != nil {
			l.WithError(err).Error("error running server")
			os.Exit(1)
		}
	} else {
		if err := pf.Run(); err != nil {
			l.WithError(err).Error("error running preflight")
			os.Exit(1)
		}
	}
}
