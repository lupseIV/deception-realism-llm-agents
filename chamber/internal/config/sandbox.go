package config

import (
	"encoding/xml"
)

type DockerRuntimeType string
const (
	DockerRuntimeRunc DockerRuntimeType = "runc"
	DockerRuntimeRunsc DockerRuntimeType = "runsc"
)

type SourceType string
const (
	SourceTypeGenerator SourceType = "generator"
	SourceTypePool 		SourceType = "pool"
)

type EnvRealismLevel string
const (
	EnvRealismLevelLow    EnvRealismLevel = "low"
	EnvRealismLevelMedium EnvRealismLevel = "medium"
	EnvRealismLevelHigh   EnvRealismLevel = "high"
)

type HoneyTokenRealismLevel string
const (
	HoneyTokenRealismLevelLow    HoneyTokenRealismLevel = "low"
	HoneyTokenRealismLevelMedium HoneyTokenRealismLevel = "medium"
	HoneyTokenRealismLevelHigh   HoneyTokenRealismLevel = "high"
)

type NetworkRealismLevel string
const (
	NetworkRealismLevelRefused NetworkRealismLevel = "refused"
	NetworkRealismLevelGeneric NetworkRealismLevel = "generic"
	NetworkRealismLevelFaithful NetworkRealismLevel = "faithful"
)

type TokenType string
const (
	TokenTypeAWSCredentials TokenType = "aws-credentials"
	TokenTypeHFToken TokenType = "hf-token"
	TokenTypeDBCredentials TokenType = "db-credentials"
)

type NetworkServiceType string
const (
	NetworkServiceTypeAWS NetworkServiceType = "aws"
)

type EgressType string
const (
	EgressTypeBlocked EgressType = "blocked"
	EgressTypeDetonation EgressType = "detonation"
)

type ID string


type Sandbox struct {
	XMLName xml.Name `xml:"sandbox"`
	ID ID `xml:"id,attr"`
	Seed *uint64 `xml:"seed,attr"`
	Base SandboxBase `xml:"base"`
	Egress EgressType `xml:"egress,attr"`
	Sources []Source `xml:"sources>source"`
	Environment Environment `xml:"environment"`
	HoneyTokens *HoneyTokens `xml:"honeytokens"`
	Network *Network `xml:"network"`
}

type Network struct {
	RealismLevel NetworkRealismLevel `xml:"realism,attr"`
	Latency ID `xml:"latency,attr"`
	Services []NetworkService `xml:"service"`
}

type NetworkService struct {
	ID ID `xml:"id,attr"`
	NetworkServiceType NetworkServiceType `xml:"type,attr"`
	Bind ID `xml:"bind,attr"`
}

type HoneyTokens struct {
	Realism HoneyTokenRealismLevel `xml:"realism,attr"`
	Tokens []Token `xml:"token"`
}

type Token struct {
	ID ID `xml:"id,attr"`
	Type TokenType `xml:"type,attr"`
	Source ID `xml:"source,attr"`
}

type SandboxBase struct {
	Image string `xml:"image,attr"`
	Runtime DockerRuntimeType `xml:"runtime,attr"`
}

type Source struct {
	ID ID `xml:"id,attr"`
	Type SourceType `xml:"type,attr"`
	File string `xml:"file,attr"`
}

type Environment struct {
	Persona string `xml:"persona,attr"`
	RealismLevel EnvRealismLevel `xml:"realism,attr"`
	AgeDays int `xml:"age-days,attr"`
}