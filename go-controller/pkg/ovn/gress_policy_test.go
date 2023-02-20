package ovn

import (
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/config"
	addressset "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/ovn/address_set"
	"github.com/stretchr/testify/assert"
	knet "k8s.io/api/networking/v1"
)

func TestGetMatchFromIPBlock(t *testing.T) {
	testcases := []struct {
		desc       string
		ipBlocks   []*knet.IPBlock
		lportMatch string
		l4Match    string
		expected   []string
	}{
		{
			desc: "IPv4 only no except",
			ipBlocks: []*knet.IPBlock{
				{
					CIDR: "0.0.0.0/0",
				},
			},
			lportMatch: "fake",
			l4Match:    "input",
			expected:   []string{"ip4.src == 0.0.0.0/0 && input && fake"},
		},
		{
			desc: "multiple IPv4 only no except",
			ipBlocks: []*knet.IPBlock{
				{
					CIDR: "0.0.0.0/0",
				},
				{
					CIDR: "10.1.0.0/16",
				},
			},
			lportMatch: "fake",
			l4Match:    "input",
			expected: []string{"ip4.src == 0.0.0.0/0 && input && fake",
				"ip4.src == 10.1.0.0/16 && input && fake"},
		},
		{
			desc: "IPv6 only no except",
			ipBlocks: []*knet.IPBlock{
				{
					CIDR: "fd00:10:244:3::49/32",
				},
			},
			lportMatch: "fake",
			l4Match:    "input",
			expected:   []string{"ip6.src == fd00:10:244:3::49/32 && input && fake"},
		},
		{
			desc: "mixed IPv4 and IPv6  no except",
			ipBlocks: []*knet.IPBlock{
				{
					CIDR: "::/0",
				},
				{
					CIDR: "0.0.0.0/0",
				},
			},
			lportMatch: "fake",
			l4Match:    "input",
			expected: []string{"ip6.src == ::/0 && input && fake",
				"ip4.src == 0.0.0.0/0 && input && fake"},
		},
		{
			desc: "IPv4 only with except",
			ipBlocks: []*knet.IPBlock{
				{
					CIDR:   "0.0.0.0/0",
					Except: []string{"10.1.0.0/16"},
				},
			},
			lportMatch: "fake",
			l4Match:    "input",
			expected:   []string{"ip4.src == 0.0.0.0/0 && ip4.src != {10.1.0.0/16} && input && fake"},
		},
		{
			desc: "multiple IPv4 with except",
			ipBlocks: []*knet.IPBlock{
				{
					CIDR:   "0.0.0.0/0",
					Except: []string{"10.1.0.0/16"},
				},
				{
					CIDR: "10.1.0.0/16",
				},
			},
			lportMatch: "fake",
			l4Match:    "input",
			expected: []string{"ip4.src == 0.0.0.0/0 && ip4.src != {10.1.0.0/16} && input && fake",
				"ip4.src == 10.1.0.0/16 && input && fake"},
		},
		{
			desc: "IPv4 with IPv4 except",
			ipBlocks: []*knet.IPBlock{
				{
					CIDR:   "0.0.0.0/0",
					Except: []string{"10.1.0.0/16"},
				},
			},
			lportMatch: "fake",
			l4Match:    "input",
			expected:   []string{"ip4.src == 0.0.0.0/0 && ip4.src != {10.1.0.0/16} && input && fake"},
		},
	}

	for _, tc := range testcases {
		gressPolicy := newGressPolicy(knet.PolicyTypeIngress, 5, "testing", "test")
		for _, ipBlock := range tc.ipBlocks {
			gressPolicy.addIPBlock(ipBlock)
		}
		output := gressPolicy.getMatchFromIPBlock(tc.lportMatch, tc.l4Match)
		assert.Equal(t, tc.expected, output)
	}
}

func TestGetL4Match(t *testing.T) {
	testcases := []struct {
		desc     string
		protocol string
		port     int32
		endPort  int32
		expected string
	}{
		{
			"unsupported protocol",
			"kube",
			0,
			0,
			"",
		},
		{
			"valid protocol with no endport specified",
			"TCP",
			300,
			0,
			"tcp && tcp.dst==300",
		},
		{
			"valid protocol with endport specified",
			"TCP",
			300,
			310,
			"tcp && 300<=tcp.dst<=310",
		},
		{
			"valid protocol with no ports specified",
			"TCP",
			0,
			0,
			"tcp",
		},
	}

	for _, tc := range testcases {
		pp := &portPolicy{
			protocol: tc.protocol,
			port:     tc.port,
			endPort:  tc.endPort,
		}
		l4Match, _ := pp.getL4Match()
		assert.Equal(t, tc.expected, l4Match)
	}
}

var _ = ginkgo.Describe("gressPolicy", func() {

	var previousIPV4Mode bool

	ginkgo.BeforeEach(func() {
		previousIPV4Mode = config.IPv4Mode
		config.IPv4Mode = true
	})

	ginkgo.AfterEach(func() {
		config.IPv4Mode = previousIPV4Mode
	})

	ginkgo.It("should manage deletiong of the same IP address from non owning Pod", func() {
		asFactory := addressset.NewFakeAddressSetFactory()
		gp := newGressPolicy(knet.PolicyTypeIngress, 0, "ns1", "pol1")
		gp.ensurePeerAddressSet(asFactory)

		p1 := newPod("ns1", "pod1", "node1", "10.10.10.10")
		p2 := newPod("ns1", "pod2", "node1", "10.10.10.10")

		gp.addPeerPods(p2)
		gp.addPeerPods(p1)
		asFactory.EventuallyExpectAddressSetWithIPs("ns1.pol1.ingress.0", []string{"10.10.10.10"})

		gp.deletePeerPod(p2)
		asFactory.EventuallyExpectAddressSetWithIPs("ns1.pol1.ingress.0", []string{"10.10.10.10"})

		gp.deletePeerPod(p1)
		asFactory.EventuallyExpectEmptyAddressSetExist("ns1.pol1.ingress.0")
	})
})
