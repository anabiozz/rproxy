// Copyright 2019 Bezrukov Alex. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rproxy

import (
	"log"
)

// LogRequestPayload the typeform payload and redirect url
func LogRequestPayload(rp string, proxyURL string) {
	log.Printf("proxy: %s, proxy_url: %s\n", rp, proxyURL)
}
