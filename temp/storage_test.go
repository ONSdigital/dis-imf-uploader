package temp

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestInMemoryStorage(t *testing.T) {
	Convey("Given an in-memory storage", t, func() {
		storage := NewInMemoryStorage()
		ctx := context.Background()

		Convey("When storing data", func() {
			data := []byte("test file content")
			err := storage.Store(ctx, "test-key", data)

			Convey("Then it should store successfully", func() {
				So(err, ShouldBeNil)
			})

			Convey("And the data should be retrievable", func() {
				retrieved, err := storage.Get(ctx, "test-key")
				So(err, ShouldBeNil)
				So(retrieved, ShouldEqual, data)
			})
		})

		Convey("When retrieving non-existent key", func() {
			_, err := storage.Get(ctx, "non-existent")

			Convey("Then it should return error", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "not found")
			})
		})

		Convey("When deleting data", func() {
			_ = storage.Store(ctx, "to-delete", []byte("data"))
			err := storage.Delete(ctx, "to-delete")

			Convey("Then delete should succeed", func() {
				So(err, ShouldBeNil)
			})

			Convey("And data should no longer exist", func() {
				_, err := storage.Get(ctx, "to-delete")
				So(err, ShouldNotBeNil)
			})
		})
	})
}
