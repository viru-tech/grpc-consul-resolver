// Package consul implements a GRPC resolver for consul service discovery.
// The resolver queries consul for healthy services with a specified name.
// [Blocking Consul queries] are used to monitor Consul for changes.
//
// To register the resolver with the GRPC-Go library run:
//
//	resolver.Register(consul.NewBuilder())
//
// Afterwards it can be used by calling [google.golang.org/grpc.Dial] and
// passing an URL in the following format:
//
//	consul://[<consul-server>]/<serviceName>[?<OPT>[&<OPT>]...]
//
// OPT is one of:
//
//   - scheme=http|https specifies if the connection to Consul is established
//     via HTTP or HTTPS.
//   - tags=<tag>[,<tag>]... only resolves to instances that have the given
//     tags. Default: empty
//   - health=healthy|fallbackToUnhealthy filters Services by their health status.
//     If set to "healthy", the service is only resolved to instances with
//     passing health checks. If set to "fallbackToUnhealthy", the service
//     resolves to all instances, if none with a passing status is available.
//     Default: healthy
//   - token=<string> includes the token in API-Requests to Consul.
//   - dc=<string> specifies DC for service search.
//
// If an OPT is defined multiple times, only the value of the last occurrence
// is used.
//
// The resolver can also be configured via the standard [Consul Environment Variables].
// The supported environment variables and their defaults depend on the version
// of the [github.com/hashicorp/consul/api] package.
//
// If consul-server, scheme or token is not specified in the URL, the settings
// defined via the [Consul Environment Variables] are used. If they are not
// defined, the defaults of the [github.com/hashicorp/consul/api.NewClient] are
// used.
//
// [Blocking Consul queries]: https://developer.hashicorp.com/consul/api-docs/features/blocking
// [Consul Environment Variables]: https://developer.hashicorp.com/consul/commands#environment-variables
package consul

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"google.golang.org/grpc/resolver"
)

type resolverBuilder struct{}

const scheme = "consul"

// NewBuilder returns a builder for a consul resolver.
func NewBuilder() resolver.Builder {
	return &resolverBuilder{}
}

func extractOpts(opts url.Values) (scheme string, tags []string, health healthFilter, token string, dc string, err error) {
	for key, values := range opts {
		if len(values) == 0 {
			continue
		}
		value := values[len(values)-1]

		switch strings.ToLower(key) {
		case "scheme":
			scheme = strings.ToLower(value)
			if scheme != "http" && scheme != "https" {
				return "", nil, healthFilterUndefined, "", "", fmt.Errorf("unsupported scheme '%s'", value)
			}
		case "tags":
			tags = strings.Split(value, ",")
		case "dc":
			dc = value
		case "health":
			switch strings.ToLower(value) {
			case "healthy":
				health = healthFilterOnlyHealthy
			case "fallbacktounhealthy":
				health = healthFilterFallbackToUnhealthy
			default:
				return "", nil, healthFilterUndefined, "", "", fmt.Errorf("unsupported health parameter value: '%s'", value)
			}
		case "token":
			token = value
		default:
			return "", nil, healthFilterUndefined, "", "", fmt.Errorf("unsupported parameter: '%s'", key)
		}
	}

	return scheme, tags, health, token, dc, err
}

func parseEndpoint(url *url.URL) (serviceName, scheme string, tags []string, health healthFilter, token string, dc string, err error) {
	const defHealthFilter = healthFilterOnlyHealthy

	// url.Path contains a leading "/", when the URL is in the form
	// scheme://host/path, remove it
	serviceName = strings.TrimPrefix(url.Path, "/")
	if serviceName == "" {
		return "", "", nil, health, "", "", errors.New("path is missing in url")
	}

	scheme, tags, health, token, dc, err = extractOpts(url.Query())
	if err != nil {
		return "", "", nil, health, "", "", err
	}

	if health == healthFilterUndefined {
		health = defHealthFilter
	}

	return serviceName, scheme, tags, health, token, dc, nil
}

func (*resolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	serviceName, scheme, tags, health, token, dc, err := parseEndpoint(&target.URL)
	if err != nil {
		return nil, err
	}

	r, err := newConsulResolver(cc, scheme, target.URL.Host, serviceName, tags, health, token, dc)
	if err != nil {
		return nil, err
	}

	r.start()

	return r, nil
}

// Scheme returns the URI scheme for the resolver
func (*resolverBuilder) Scheme() string {
	return scheme
}
