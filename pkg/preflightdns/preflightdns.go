package preflightdns

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var (
	Logger *log.Logger
)

func init() {
	if Logger == nil {
		Logger = log.New()
		Logger.SetOutput(os.Stdout)
		Logger.SetLevel(log.InfoLevel)
	}
}

type ConnectionState struct {
	StatusCode int
}

type PreflightDNS struct {
	Endpoint      string            `json:"endpoint" yaml:"endpoint"`
	Headers       map[string]string `json:"headers" yaml:"headers"`
	Body          string            `json:"body" yaml:"body"`
	Method        string            `json:"method" yaml:"method"`
	New           string            `json:"new" yaml:"new"`
	Timeout       time.Duration     `json:"timeout" yaml:"timeout"`
	LowerIsBetter bool              `json:"lowerIsBetter" yaml:"lowerIsBetter"`
	Equiv         bool              `json:"equiv" yaml:"equiv"`

	currentState ConnectionState
	newState     ConnectionState
}

func LoadConfig(filepath string) (*PreflightDNS, error) {
	l := Logger.WithFields(log.Fields{
		"fn": "LoadConfig",
	})
	l.Debug("loading config")
	var err error
	pf := &PreflightDNS{}
	bd, err := os.ReadFile(filepath)
	if err != nil {
		l.WithError(err).Error("error reading file")
		return pf, err
	}
	if err := yaml.Unmarshal(bd, pf); err != nil {
		// try with json
		if err := json.Unmarshal(bd, pf); err != nil {
			l.WithError(err).Error("error unmarshalling config")
			return pf, err
		}
	}
	return pf, err
}

func (d *PreflightDNS) Init() error {
	l := Logger.WithFields(log.Fields{
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
	l := Logger.WithFields(log.Fields{
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
	l := Logger.WithFields(log.Fields{
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
	l := Logger.WithFields(log.Fields{
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

func (d *PreflightDNS) EquivalentCmd() {
	l := Logger
	l.Debug("printing equivalent command")
	var cmd string
	timeoutSeconds := int(d.Timeout.Seconds())
	cmd = fmt.Sprintf(`ENDPOINT=%s; INPUT=%s; NEW_IP=""; if [[ $INPUT =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then NEW_IP=$INPUT; else NEW_IP=$(dig +short $INPUT | tail -n1); fi; `, d.Endpoint, d.New)
	cmd += `if [[ $ENDPOINT =~ ^https:// ]]; then HOST=$(echo $ENDPOINT | sed -e "s,https://\([^/]*\).*,\1,"); else HOST=$(echo $ENDPOINT | sed -e "s,http://\([^/]*\).*,\1,"); fi;`
	cmd += `if [[ $HOST =~ :[0-9]+ ]]; then PORT=$(echo $HOST | sed -e "s,.*:\([0-9]*\).*,\1,"); HOST=$(echo $HOST | sed -e "s,:\([0-9]*\)\$,,"); fi;`
	cmd += `if [[ $ENDPOINT =~ :[0-9]+ ]]; then PORT=$(echo $ENDPOINT | sed -e "s,.*:\([0-9]*\).*,\1,"); else if [[ $ENDPOINT =~ ^https:// ]]; then PORT=443; else PORT=80; fi; fi;`
	cmd += `HEADER_STR=""; BODY_STR="";`
	for k, v := range d.Headers {
		cmd += fmt.Sprintf(`HEADER_STR="%s -H '%s: %s'"; `, cmd, k, v)
	}
	if d.Body != "" {
		cmd += fmt.Sprintf(`BODY_STR="-d '%s'"; `, d.Body)
	}
	cmd += fmt.Sprintf(`ORIG=$(curl -s -o /dev/null -m %d -w "%%{http_code}" -X %s $HEADER_STR $BODY_STR $ENDPOINT); `, timeoutSeconds, d.Method)
	cmd += fmt.Sprintf(`NEW=$(curl -s -o /dev/null -m %d -w "%%{http_code}" --resolve $HOST:$PORT:$NEW_IP "$ENDPOINT");`, timeoutSeconds)
	if d.LowerIsBetter {
		cmd += `if [[ $ORIG -gt $NEW ]]; then echo "passed"; else echo "failed" && exit 1; fi;`
	} else {
		cmd += `if [[ $ORIG -eq $NEW ]]; then echo "passed"; else echo "failed" && exit 1; fi;`
	}
	cmd = fmt.Sprintf(`sh -c '%s'`, cmd)
	fmt.Println(cmd)
}

func (d *PreflightDNS) Run() error {
	l := Logger.WithFields(log.Fields{
		"preflight": "dns",
	})
	l.Debug("running")
	err := d.Init()
	if err != nil {
		l.WithError(err).Error("error initializing")
		return err
	}
	if d.Equiv {
		d.EquivalentCmd()
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
		failstr := fmt.Sprintf("failed - expected: %d, got: %d", d.currentState.StatusCode, d.newState.StatusCode)
		l.Error(failstr)
		return errors.New(failstr)
	}
	l.Info("passed")
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
	l := Logger.WithFields(log.Fields{
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
