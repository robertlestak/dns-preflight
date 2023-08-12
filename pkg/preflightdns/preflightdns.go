package preflightdns

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

type ConnectionState struct {
	StatusCode int
}

type PreflightDNS struct {
	Endpoint      string            `json:"endpoint"`
	Headers       map[string]string `json:"headers"`
	Body          string            `json:"body"`
	Method        string            `json:"method"`
	New           string            `json:"new"`
	Timeout       time.Duration     `json:"timeout"`
	LowerIsBetter bool              `json:"lower_is_better"`

	currentState ConnectionState
	newState     ConnectionState
}

func (d *PreflightDNS) Init() error {
	l := log.WithFields(log.Fields{
		"fn": "Init",
	})
	l.Debug("initializing")
	var err error
	if d.Method == "" {
		d.Method = "GET"
	}
	if d.Headers == nil {
		d.Headers = make(map[string]string)
	}
	if d.Endpoint == "" {
		return errors.New("no endpoint provided")
	}
	if d.New == "" {
		return errors.New("no new ip provided")
	}
	if d.Timeout == 0 {
		d.Timeout = 5 * time.Second
	}
	l.Debugf("initialized: %+v", d)
	return err
}

func (d *PreflightDNS) GetCurrent() (ConnectionState, error) {
	l := log.WithFields(log.Fields{
		"fn": "GetCurrent",
	})
	l.Debug("getting current state")
	var cs ConnectionState
	var err error
	var bodyData []byte
	if d.Body != "" {
		bodyData = []byte(d.Body)
	}
	req, err := http.NewRequest(d.Method, d.Endpoint, bytes.NewBuffer(bodyData))
	if err != nil {
		l.WithError(err).Error("error creating request")
		return cs, err
	}
	for k, v := range d.Headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{}
	client.Timeout = time.Duration(d.Timeout)
	resp, err := client.Do(req)
	if err != nil {
		l.WithError(err).Error("error making request")
		return cs, err
	}
	defer resp.Body.Close()
	cs.StatusCode = resp.StatusCode
	l.WithFields(log.Fields{
		"status": resp.Status,
	}).Debug("got current state")
	d.currentState = cs
	return cs, err
}

func (d *PreflightDNS) GetNew() (ConnectionState, error) {
	l := log.WithFields(log.Fields{
		"fn": "GetNew",
	})
	l.Debug("getting new state")
	if err := d.resolveNew(); err != nil {
		l.WithError(err).Error("error resolving new ip")
		return ConnectionState{}, err
	}
	var cs ConnectionState
	var err error
	var bodyData []byte
	if d.Body != "" {
		bodyData = []byte(d.Body)
	}
	dialer := &net.Dialer{
		Timeout:  d.Timeout,
		Deadline: time.Now().Add(d.Timeout),
	}
	transport := &http.Transport{
		DialContext: dialer.DialContext,
	}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		// rewrite the address
		addr = net.JoinHostPort(d.New, port)
		return dialer.DialContext(ctx, network, addr)
	}
	client := &http.Client{
		Transport: transport,
	}
	client.Timeout = time.Duration(d.Timeout)
	req, err := http.NewRequest(d.Method, d.Endpoint, bytes.NewBuffer(bodyData))
	if err != nil {
		l.WithError(err).Error("error creating request")
		return cs, err
	}
	for k, v := range d.Headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		l.WithError(err).Error("error making request")
		return cs, err
	}
	defer resp.Body.Close()
	cs.StatusCode = resp.StatusCode
	l.WithFields(log.Fields{
		"status": resp.Status,
	}).Debug("got new state")
	d.newState = cs
	return cs, err
}

func (d *PreflightDNS) Compare() (bool, error) {
	l := log.WithFields(log.Fields{
		"fn": "Compare",
	})
	l.Debug("comparing current and new states")
	var err error
	var match bool
	if d.LowerIsBetter {
		if d.currentState.StatusCode > d.newState.StatusCode {
			match = true
		}
	}
	if d.currentState.StatusCode == d.newState.StatusCode {
		match = true
	}
	return match, err
}

func (d *PreflightDNS) Run() error {
	l := log.WithFields(log.Fields{
		"fn": "Run",
	})
	l.Debug("running")
	err := d.Init()
	if err != nil {
		l.WithError(err).Error("error initializing")
		return err
	}
	_, err = d.GetCurrent()
	if err != nil {
		l.WithError(err).Error("error getting current state")
		return err
	}
	_, err = d.GetNew()
	if err != nil {
		l.WithError(err).Error("error getting new state")
		return err
	}
	match, err := d.Compare()
	if err != nil {
		l.WithError(err).Error("error comparing states")
		return err
	}
	if !match {
		l.Error("preflight failed")
		return errors.New("preflight failed")
	}
	l.Info("preflight passed")
	return err
}

func ipForDomain(domain string) (string, error) {
	if domain == "localhost" {
		return "127.0.0.1", nil
	}
	addr := net.ParseIP(domain)
	if addr != nil {
		return domain, nil
	} else {
		ips, err := net.LookupIP(domain)
		if err != nil {
			return "", err
		}
		for _, ip := range ips {
			if ip.To4() != nil {
				return ip.String(), nil
			}
		}
	}
	return "", errors.New("no A record found")
}

func (d *PreflightDNS) resolveNew() error {
	l := log.WithFields(log.Fields{
		"fn": "resolveNew",
	})
	l.Debug("resolving new ip")
	var err error
	if d.New == "" {
		return errors.New("no new ip provided")
	}
	ip, err := ipForDomain(d.New)
	if err != nil {
		return err
	}
	d.New = ip
	return err
}
