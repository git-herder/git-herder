// Хост ремота: hostname и порт для критерия NoRemoteOnHost.
package workspace

import (
	"fmt"
	"net/url"
	"strings"
)

// Host — hostname и порт. Схема и путь в хост не входят.
type Host struct {
	Name string
	Port string
}

// HostOf разбирает значение хоста SCM.
// Hostname api.github.com приводится к github.com без порта.
func HostOf(raw string) (Host, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" {
		return Host{}, fmt.Errorf("invalid host %q", raw)
	}
	if u.Hostname() == "api.github.com" {
		return Host{Name: "github.com"}, nil
	}
	return Host{Name: u.Hostname(), Port: u.Port()}, nil
}

// SameHost сравнивает hostname и порт точно, с учётом регистра.
func SameHost(a, b Host) bool {
	return a.Name == b.Name && a.Port == b.Port
}

// hostOfRemote достаёт hostname и порт из URL ремота.
// Схема scp (git@host:path) порта не имеет. Неразбираемая строка не совпадает.
func hostOfRemote(raw string) (Host, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Host{}, false
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			return Host{}, false
		}
		return Host{Name: u.Hostname(), Port: u.Port()}, true
	}
	rest := raw
	if at := strings.LastIndex(raw, "@"); at >= 0 {
		rest = raw[at+1:]
	}
	colon := strings.Index(rest, ":")
	if colon <= 0 {
		return Host{}, false
	}
	host := rest[:colon]
	if host == "" || strings.Contains(host, "/") {
		return Host{}, false
	}
	return Host{Name: host}, true
}
