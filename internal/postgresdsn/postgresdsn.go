package postgresdsn

import (
	"fmt"
	"net/url"
)

func Build(host string, port int, username, password, database, sslMode string) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(username, password),
		Host:   fmt.Sprintf("%s:%d", host, port),
		Path:   database,
	}

	query := url.Values{}
	query.Set("sslmode", sslMode)
	u.RawQuery = query.Encode()

	return u.String()
}
