package main

import "github.com/xs-memory/xs-memory/xsmem"

// openStore opens the store at the configured path with default options.
func openStore(opts ...xsmem.Option) (*xsmem.Store, error) {
	return xsmem.Open(storePath, opts...)
}
