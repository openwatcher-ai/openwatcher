package server

import "net/http"

func watcherHeaderValue(header http.Header, suffix string) string {
	return header.Get("X-OpenWatcher-" + suffix)
}
