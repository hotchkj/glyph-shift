//go:build mage
// +build mage

package main

import (
	"testing"

	"github.com/hotchkj/mage-gate/gatetest"
)

func rootedOptionsMemoryFileOps(tb testing.TB) *gatetest.MemoryFileOps {
	tb.Helper()
	mem := gatetest.NewMemoryFileOps()
	if rootErr := mem.Root(testFakeModuleRoot); rootErr != nil {
		tb.Fatal(rootErr)
	}
	return mem
}
