//go:build !production

package main

import "net/http"

func frontendFS() http.FileSystem { return nil }
