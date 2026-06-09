package domain

import "fmt"

// ID monta https://{host}/{collection}/{name}/{version}
func ID(host, collection, name, version string) string {
	if version == "" {
		return fmt.Sprintf("https://%s/%s/%s", host, collection, name)
	}
	return fmt.Sprintf("https://%s/%s/%s/%s", host, collection, name, version)
}
