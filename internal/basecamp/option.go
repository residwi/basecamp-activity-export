package basecamp

import "net/http"

type optionFunc func(*Client)

func WithHTTPClient(httpClient *http.Client) optionFunc {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

func WithAccountID(id string) optionFunc {
	return func(c *Client) {
		c.accountID = id
	}
}

func WithAccessToken(token string) optionFunc {
	return func(c *Client) {
		c.accessToken = token
	}
}

func WithUserAgent(ua string) optionFunc {
	return func(c *Client) {
		c.userAgent = ua
	}
}

func WithBaseURL(url string) optionFunc {
	return func(c *Client) {
		c.baseURL = url
	}
}

func WithConcurrencyLimit(limit int) optionFunc {
	return func(c *Client) {
		c.semaphore = make(chan struct{}, limit)
	}
}
