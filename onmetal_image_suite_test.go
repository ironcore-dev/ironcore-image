package onmetal_image_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOnmetalImage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OnmetalImage Suite")
}
