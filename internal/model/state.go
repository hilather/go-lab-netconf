package model

const (
	// APIVersionV1Alpha1 is the only 1.0 config API version.
	APIVersionV1Alpha1 = "labnetconf.dev/v1alpha1"
	// KindLabNETCONF is the config document kind.
	KindLabNETCONF = "LabNETCONF"
)

// State is the canonical desired-state document.
type State struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   Metadata `json:"metadata"`
	Spec       Spec     `json:"spec"`
}

// Metadata is document identity and labels.
type Metadata struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
}

// Spec is the v1alpha1 desired-state contract. YAML decode and default
// materialization live in config, not here.
type Spec struct {
	Listeners  ListenersSpec  `json:"listeners"`
	Auth       AuthSpec       `json:"auth"`
	UI         UISpec         `json:"ui"`
	Netconf    NetconfSpec    `json:"netconf"`
	Restconf   RestconfSpec   `json:"restconf"`
	Admission  AdmissionSpec  `json:"admission"`
	Management ManagementSpec `json:"management"`
	Profiles   []ProfileSpec  `json:"profiles"`
	Users      []UserSpec     `json:"users"`
}
