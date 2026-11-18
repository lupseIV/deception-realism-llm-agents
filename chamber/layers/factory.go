// Package layers registers every layer implementation in one factory.
package layers

import (
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/assemble"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/internal/master"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/layers/environment/persona"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/layers/honeytoken/awscreds"
	"github.com/lupseIV/deception-realism-llm-agents/chamber/layers/network/aws"
)

// Factory returns the factory with all implemented layers. backend is the
// Moto URL used by AWS network layers in faithful mode.
func Factory(backend string) assemble.Factory {
	return assemble.Factory{
		persona.Kind: func(s assemble.LayerSpec) (master.Layer, error) {
			return persona.New(s)
		},
		awscreds.Kind: func(s assemble.LayerSpec) (master.Layer, error) {
			return awscreds.New(s)
		},
		aws.Kind: func(s assemble.LayerSpec) (master.Layer, error) {
			return aws.New(s, aws.WithBackend(backend))
		},
	}
}
