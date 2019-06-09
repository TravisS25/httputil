package apiutil

import "strings"

func ReplaceURL(url string, replacements map[string]string) string {
	for k, v := range replacements {
		url = strings.Replace(url, k, v, 1)
	}

	return url
}
