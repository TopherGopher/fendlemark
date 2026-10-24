package main

// The container image is built FROM scratch and has no root certificates of
// its own. When the system has none, TLS falls back to the Mozilla roots
// bundled here; when it has some, they are used instead.
import _ "golang.org/x/crypto/x509roots/fallback"
