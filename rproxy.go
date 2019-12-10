// Copyright 2019 Bezrukov Alex. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rproxy

import "os"

/**
==================       ==================
=                =       =                =
=                =       =                =
=    PROVIDER    = ====> =     ROUTER     =
=                =       =                =
=                =       =                =
==================       ==================
*/

// Proxer ..
type Proxer interface {
	Proxy()
}

// GetEnv ..
func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
