// Пакет http: ScmHttpTransport — обёртка GET-запросов к SCM REST API.
package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const userAgent = "githerd"

// Response — результат одного HTTP-запроса к SCM.
type Response struct {
	Status     int
	Body       []byte
	Header     http.Header
	RequestURL string
}

// Transport выполняет авторизованные GET-запросы к SCM API.
type Transport struct {
	client *http.Client
	token  string
}

// NewTransport создаёт transport с заданным клиентом и bearer-токеном.
func NewTransport(client *http.Client, token string) *Transport {
	if client == nil {
		client = http.DefaultClient
	}
	return &Transport{client: client, token: strings.TrimSpace(token)}
}

// Client возвращает HTTP-клиент transport (для клонирования с новым токеном).
func (t *Transport) Client() *http.Client {
	return t.client
}

// Get выполняет GET baseURL+path с query-параметрами.
func (t *Transport) Get(ctx context.Context, baseURL, path string, query url.Values) (Response, error) {
	base := strings.TrimRight(baseURL, "/")
	p := strings.TrimLeft(path, "/")
	rawURL := base + "/" + p
	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}
	return Response{
		Status:     resp.StatusCode,
		Body:       body,
		Header:     resp.Header.Clone(),
		RequestURL: rawURL,
	}, nil
}

// BaseURL возвращает корень REST API для host и upstream.
func BaseURL(host, upstream string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", fmt.Errorf("scm host is empty")
	}
	switch strings.ToLower(strings.TrimSpace(upstream)) {
	case "github":
		if strings.EqualFold(host, "github.com") {
			return "https://api.github.com", nil
		}
		return "https://" + host + "/api", nil
	case "gitlab":
		return "https://" + host + "/api/v4", nil
	default:
		return "", fmt.Errorf("unknown upstream %q", upstream)
	}
}
