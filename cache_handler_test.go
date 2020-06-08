package yarninstall_test

import (
	"testing"

	yarninstall "github.com/paketo-buildpacks/yarn-install"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testCacheHandler(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		cacheHandler yarninstall.CacheHandler
	)

	it.Before(func() {
		cacheHandler = yarninstall.NewCacheHandler()
	})

	context("Match", func() {
		var metadata map[string]interface{}

		it.Before(func() {
			metadata = map[string]interface{}{
				"cache-sha": "some-sha",
			}
		})

		context("when the layer metadata and choosen dependency shas match", func() {
			it("it returns true and no error", func() {
				match := cacheHandler.Match(metadata, "cache-sha", "some-sha")
				Expect(match).To(BeTrue())
			})
		})

		context("when the layer metadata and choosen dependency shas do not match", func() {
			it("it returns false and no error", func() {
				match := cacheHandler.Match(metadata, "cache-sha", "other-sha")
				Expect(match).To(BeFalse())
			})
		})

		context("when the layer metadata does not contain the dependency-sha", func() {
			it("it returns false and no error", func() {
				match := cacheHandler.Match(map[string]interface{}{}, "cache-sha", "some-sha")
				Expect(match).To(BeFalse())
			})
		})

		context("when the cache key has a type mismatch", func() {
			it.Before(func() {
				metadata["cache-sha"] = 10
			})

			it("returns false", func() {
				match := cacheHandler.Match(metadata, "cache-sha", "some-sha")
				Expect(match).To(BeFalse())
			})
		})
	})

}
