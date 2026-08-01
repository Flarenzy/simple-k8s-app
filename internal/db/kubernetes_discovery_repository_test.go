package db

import (
	"net/netip"
	"testing"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
)

func TestKubernetesServiceMatchStatus(t *testing.T) {
	tests := []struct {
		name      string
		addresses []domain.KubernetesAddressObservation
		want      domain.KubernetesMatchStatus
	}{
		{name: "no usable IP", want: domain.KubernetesMatchNoUsableIP},
		{name: "unmatched", addresses: []domain.KubernetesAddressObservation{{IP: netip.MustParseAddr("10.0.0.1"), MatchStatus: domain.KubernetesMatchUnmatched}}, want: domain.KubernetesMatchUnmatched},
		{name: "ambiguous", addresses: []domain.KubernetesAddressObservation{{IP: netip.MustParseAddr("10.0.0.1"), MatchStatus: domain.KubernetesMatchUnmatched}, {IP: netip.MustParseAddr("10.0.0.2"), MatchStatus: domain.KubernetesMatchAmbiguous}}, want: domain.KubernetesMatchAmbiguous},
		{name: "matched takes precedence", addresses: []domain.KubernetesAddressObservation{{IP: netip.MustParseAddr("10.0.0.1"), MatchStatus: domain.KubernetesMatchAmbiguous}, {IP: netip.MustParseAddr("10.0.0.2"), MatchStatus: domain.KubernetesMatchMatched}}, want: domain.KubernetesMatchMatched},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := kubernetesServiceMatchStatus(test.addresses); got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}
