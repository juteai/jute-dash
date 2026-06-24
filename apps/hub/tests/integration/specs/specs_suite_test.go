package specs

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHubIntegrationSpecs(t *testing.T) {
	if os.Getenv("JUTE_HUB_INTEGRATION") != "1" {
		t.Skip("set JUTE_HUB_INTEGRATION=1 and start a hub to run integration specs")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hub Integration Specs")
}
