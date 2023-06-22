package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	corev2 "github.com/sensu/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	PrometheusUrl      string
	InsecureSkipVerify bool
	TrustedCAFile      string
	Timeout            int
	FiringAlerts       bool
	PendingAlerts      bool
	Labels             map[string]string
	Annotations        map[string]string
	VerboseLogging     bool
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "sensu-prometheus-alert-check",
			Short:    "A Sensu check for monitoring alerts in Prometheus.",
			Keyspace: "sensu.io/plugins/sensu-prometheus-alert-check/config",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "prometheus-url",
			Env:       "PROMETHEUS_URL",
			Argument:  "url",
			Shorthand: "u",
			Default:   "http://127.0.0.1:9090/",
			Usage:     "The base path of the Prometheus API.",
			Value:     &plugin.PrometheusUrl,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "insecure-skip-verify",
			Env:       "PROMETHEUS_SKIP_VERIFY",
			Argument:  "insecure-skip-verify",
			Shorthand: "i",
			Default:   false,
			Usage:     "Skip TLS certificate verification (not recommended!)",
			Value:     &plugin.InsecureSkipVerify,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "trusted-ca-file",
			Env:       "PROMETHEUS_CACERT",
			Argument:  "trusted-ca-file",
			Shorthand: "T",
			Default:   "",
			Usage:     "TLS CA certificate bundle in PEM format",
			Value:     &plugin.TrustedCAFile,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "timeout",
			Env:       "PROMETHEUS_TIMEOUT",
			Argument:  "timeout",
			Shorthand: "t",
			Default:   15,
			Usage:     "The number of seconds the test should wait for a response form the host (Default: 15).",
			Value:     &plugin.Timeout,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "firing",
			Env:       "",
			Argument:  "firing",
			Shorthand: "f",
			Default:   false,
			Usage:     "If specified, the check will only look for firing alerts.",
			Value:     &plugin.FiringAlerts,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "pending",
			Env:       "",
			Argument:  "pending",
			Shorthand: "p",
			Default:   false,
			Usage:     "If specified, the check will only look for pending alerts.",
			Value:     &plugin.PendingAlerts,
		},
		&sensu.MapPluginConfigOption[string]{
			Path:      "labels",
			Env:       "",
			Argument:  "label",
			Shorthand: "l",
			Default:   make(map[string]string, 0),
			Usage:     "Filter alerts by labels using a RegEx. Can be specified more than once. E.g. '--label myname=\"(Alert1|Alert2|^$)\"' will match 'Alert1', 'Alert2', and alerts with no label called 'myname'.",
			Value:     &plugin.Labels,
		},
		&sensu.MapPluginConfigOption[string]{
			Path:      "annotations",
			Env:       "",
			Argument:  "annotation",
			Shorthand: "a",
			Default:   make(map[string]string, 0),
			Usage:     "Filter alerts by annotations using a RegEx. Can be specified more than once.",
			Value:     &plugin.Annotations,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "verbose",
			Env:       "",
			Argument:  "verbose",
			Shorthand: "v",
			Default:   false,
			Usage:     "If specified, output will be more verbose.",
			Value:     &plugin.VerboseLogging,
		},
	}
)

func main() {
	useStdin := false
	fi, err := os.Stdin.Stat()
	if err != nil {
		fmt.Printf("Error check stdin: %v\n", err)
		panic(err)
	}
	//Check the Mode bitmask for Named Pipe to indicate stdin is connected
	if fi.Mode()&os.ModeNamedPipe != 0 {
		log.Println("using stdin")
		useStdin = true
	}

	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, useStdin)
	check.Execute()
}

func checkArgs(event *corev2.Event) (int, error) {
	if len(plugin.PrometheusUrl) == 0 {
		return sensu.CheckStateWarning, fmt.Errorf("--url or PROMETHEUS_URL environment variable is required")
	}

	_, err := url.Parse(plugin.PrometheusUrl)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("failed to parse prometheus URL %s: %v", plugin.PrometheusUrl, err)
	}

	if plugin.PendingAlerts && plugin.FiringAlerts {
		return sensu.CheckStateWarning, fmt.Errorf("both --pending and --firing cannot be specified at the same time")
	}

	return sensu.CheckStateOK, nil
}

func executeCheck(event *corev2.Event) (int, error) {
	log.Printf("Executing check with --url \"%s\".\n", plugin.PrometheusUrl)

	baseUrl, err := url.Parse(plugin.PrometheusUrl)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("failed to parse base URL %s: %v", plugin.PrometheusUrl, err)
	}

	client, err := GetHttpClient(baseUrl)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("unable to build HTTP client: %v", err)
	}

	log.Println("Fetching alerts...")
	alerts, err := GetAlerts(client, baseUrl)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("unable to fetch alerts: %v", err)
	}

	alerts, err = FilterAlerts(alerts)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("unable filter alerts: %v", err)
	}

	if len(alerts) > 0 {
		log.Printf("%d FOUND ALERTS:\n", len(alerts))

		bytes, err := json.MarshalIndent(alerts, "", "  ")
		if err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("unable to marshal alerts: %v", err)
		}

		fmt.Println(string(bytes))
		return sensu.CheckStateCritical, nil
	}

	log.Println("Check passed, no alerts found.")
	return sensu.CheckStateOK, nil
}

func CopyToEvent(names []string, src map[string]string, dest map[string]string) {
	for _, name := range names {
		value, ok := src[name]
		if ok {
			dest[name] = value
		}
	}
}

func CompileFilters(exps map[string]string) (map[string]regexp.Regexp, error) {
	result := make(map[string]regexp.Regexp, len(exps))

	for name, regex := range exps {
		LogVerbosef("Compiling filter %s=%s\n", name, regex)
		xp, err := regexp.Compile(regex)

		if err != nil {
			return nil, fmt.Errorf("failed to compile regex '%s': %v", regex, err)
		}

		result[name] = *xp
	}

	return result, nil
}

func MatchProperty(values map[string]string, filters map[string]regexp.Regexp) (bool, *string, *string) {
	result := true

	for name, regex := range filters {
		value, ok := values[name]

		if ok {
			result = regex.MatchString(value)
		} else {
			result = regex.MatchString("")
		}

		if !result {
			return result, &name, &value
		}
	}

	return result, nil, nil
}

func FilterAlerts(alerts []Alert) ([]Alert, error) {
	labelFilters, err := CompileFilters(plugin.Labels)
	if err != nil {
		return nil, fmt.Errorf("failed to compile label filter: %v", err)
	}

	annotationFilters, err := CompileFilters(plugin.Annotations)
	if err != nil {
		return nil, fmt.Errorf("failed to compile label filter: %v", err)
	}

	result := []Alert{}

	for _, alert := range alerts {
		match := true
		var name *string
		var value *string

		match = match && (!plugin.FiringAlerts || alert.State == "firing")
		match = match && (!plugin.PendingAlerts || alert.State == "pending")

		if !match {
			LogVerbosef("Alert ignored due to state: %s.\n", alert.State)
		}

		if match {
			match, name, value = MatchProperty(alert.Labels, labelFilters)

			if !match {
				LogVerbosef("Alert ignored due to label: %s=%s.\n", *name, *value)
			}
		}

		if match {
			match, name, value = MatchProperty(alert.Annotations, annotationFilters)

			if !match {
				LogVerbosef("Alert ignored due to annotation: %s=%s.\n", *name, *value)
				break
			}
		}

		if match {
			result = append(result, alert)
		}
	}

	return result, nil
}

func GetAlerts(client *http.Client, baseUrl *url.URL) ([]Alert, error) {
	url := baseUrl.String() + "/api/v1/alerts"
	log.Printf("Executing request \"GET %s\".\n", url)
	req, err := http.NewRequest(http.MethodGet, url, nil)

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	bytes, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("failed to read body of response: %v", err)
	}

	var data Alerts
	err = json.Unmarshal(bytes, &data)

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return data.Data.Alerts, nil
}

type Alert struct {
	State       string            `json:"state"`
	ActiveAt    string            `json:"activeAt"`
	Value       string            `json:"value"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type Alerts struct {
	Status string `json:"status"`
	Data   struct {
		Alerts []Alert `json:"alerts"`
	} `json:"data"`
}

func GetHttpClient(url *url.URL) (*http.Client, error) {
	client := http.DefaultClient
	client.Timeout = time.Duration(plugin.Timeout) * time.Second
	tr := http.DefaultTransport

	if url.Scheme == "https" {
		var caCertPool *x509.CertPool = nil

		if len(plugin.TrustedCAFile) > 0 {
			caCert, err := ioutil.ReadFile(plugin.TrustedCAFile)
			if err != nil {
				return nil, fmt.Errorf("unable to read cert file at %s: %v", plugin.TrustedCAFile, err)
			}
			caCertPool = x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
		}

		tr = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: plugin.InsecureSkipVerify,
				RootCAs:            caCertPool,
			},
		}
	}

	client.Transport = tr
	return client, nil
}

func LogVerbosef(format string, v ...any) {
	if plugin.VerboseLogging {
		log.Printf(format, v...)
	}
}

func LogVerboseln(v ...any) {
	if plugin.VerboseLogging {
		log.Println(v...)
	}
}
