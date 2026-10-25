package config

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// RedactedConfigMarker starts a redacted config file, so it can't be imported by mistake
const RedactedConfigMarker = "# aura redacted configuration"

// RedactedValue replaces secrets and personal values in a redacted config
const RedactedValue = "<redacted>"

// Keys whose values are secrets or identify the user
var redactedKeyPattern = regexp.MustCompile(`(?i)(token|secret|password|apikey|userkey|hash|webhook|clientid|userid|allowedemails|alloweddomains)`)

var (
	ipv4Pattern  = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	// A url in free text ends at a space, a quote or an escaped quote
	urlPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s"'<>\\]+`)
)

// Shortest secret value that is also replaced in other text, so short values don't replace common words
const minRedactedSecretLength = 6

// RedactConfigYAML replaces the secrets, emails and addresses in a config file, so it can be shared.
// Each address becomes host-1, host-2 and so on, the same address always gets the same name.
// Empty values are kept, so what isn't set still shows.
func RedactConfigYAML(data []byte) ([]byte, error) {
	return NewRedactor().ConfigYAML(data)
}

// Redactor replaces secrets, emails and addresses. An address gets the same name in everything it redacts.
type Redactor struct {
	hosts   map[string]string
	secrets []string
}

func NewRedactor() *Redactor {
	return &Redactor{hosts: map[string]string{}}
}

// ConfigYAML redacts a config file, and keeps its secrets so Text replaces them too
func (r *Redactor) ConfigYAML(data []byte) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	r.node(&doc, false)
	return yaml.Marshal(&doc)
}

// Text redacts free text such as log lines: the secrets and host names from the config, then urls, emails and addresses
func (r *Redactor) Text(value string) string {
	for _, secret := range r.secrets {
		value = strings.ReplaceAll(value, secret, RedactedValue)
	}
	value = urlPattern.ReplaceAllStringFunc(value, r.text)
	// Host names from the config can also appear without a url
	for _, host := range r.hostNames() {
		value = strings.ReplaceAll(value, host, r.hosts[host])
	}
	value = emailPattern.ReplaceAllString(value, "redacted@redacted")
	return ipv4Pattern.ReplaceAllStringFunc(value, r.host)
}

// hostNames returns the names that aren't addresses, longest first so a name isn't replaced inside a longer one
func (r *Redactor) hostNames() []string {
	names := make([]string, 0, len(r.hosts))
	for host := range r.hosts {
		if net.ParseIP(host) == nil {
			names = append(names, host)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		if len(names[i]) != len(names[j]) {
			return len(names[i]) > len(names[j])
		}
		return names[i] < names[j]
	})
	return names
}

// node redacts a YAML node, secret is whether a parent key marks everything below it as secret
func (r *Redactor) node(n *yaml.Node, secret bool) {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range n.Content {
			r.node(child, secret)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, value := n.Content[i], n.Content[i+1]
			// Header values often hold credentials
			childSecret := secret || strings.EqualFold(key.Value, "Headers") ||
				(value.Kind != yaml.MappingNode && redactedKeyPattern.MatchString(key.Value))
			r.node(value, childSecret)
		}
	case yaml.ScalarNode:
		if n.Tag != "!!str" || n.Value == "" {
			return
		}
		if secret {
			if len(n.Value) >= minRedactedSecretLength && !slices.Contains(r.secrets, n.Value) {
				r.secrets = append(r.secrets, n.Value)
			}
			n.Value = RedactedValue
			return
		}
		n.Value = r.text(n.Value)
	}
}

// text replaces the emails and addresses in a value
func (r *Redactor) text(value string) string {
	value = emailPattern.ReplaceAllString(value, "redacted@redacted")
	if u, err := url.Parse(value); err == nil && u.Scheme != "" && u.Host != "" {
		u.User = nil
		// Query values can hold tokens
		if u.RawQuery != "" {
			u.RawQuery = "redacted"
		}
		if port := u.Port(); port != "" {
			u.Host = net.JoinHostPort(r.host(u.Hostname()), port)
		} else {
			u.Host = r.host(u.Hostname())
		}
		return u.String()
	}
	return ipv4Pattern.ReplaceAllStringFunc(value, r.host)
}

// Public services aura uses, kept so their errors still show which service failed
var publicHosts = []string{"mediux.io", "mediux.pro", "themoviedb.org", "tmdb.org", "plex.tv", "github.com", "githubusercontent.com", "discord.com", "pushover.net"}

// host returns the shared name for an address, loopback addresses and public services are kept
func (r *Redactor) host(host string) string {
	lower := strings.ToLower(host)
	if lower == "localhost" || lower == "::1" || strings.HasPrefix(lower, "127.") {
		return host
	}
	for _, public := range publicHosts {
		if lower == public || strings.HasSuffix(lower, "."+public) {
			return host
		}
	}
	if name, ok := r.hosts[lower]; ok {
		return name
	}
	name := fmt.Sprintf("host-%d", len(r.hosts)+1)
	r.hosts[lower] = name
	return name
}
