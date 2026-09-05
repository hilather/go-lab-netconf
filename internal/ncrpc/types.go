package ncrpc

// Message is one decoded NETCONF XML document.
type Message struct {
	Hello *Hello
	RPC   *RPC
	Reply *Reply
}

// Hello is a <hello> element. SessionID is empty on a client hello.
type Hello struct {
	Capabilities []string
	SessionID    string
}

// RPC is a <rpc> request. MessageID is the RFC 6241 message-id attribute.
type RPC struct {
	MessageID string
	Name      string
	Namespace string
	Source    string
	Target    string
	Filter    *Filter
	DefaultOp string
	Config    []byte
	SessionID string
	Stream    string
}

// Filter is a <filter> element. Type is subtree or xpath.
type Filter struct {
	Type   string
	Select string
	Inner  []byte
}

// Reply is a <rpc-reply> element.
type Reply struct {
	MessageID string
	OK        bool
	Data      []byte
	Errors    []Error
}

// Error is one <rpc-error> in a reply.
type Error struct {
	Type     string
	Tag      string
	Severity string
	AppTag   string
	Path     string
	Message  string
}

// RFC 6241 / RFC 5277 operation local names.
const (
	OpGet                = "get"
	OpGetConfig          = "get-config"
	OpEditConfig         = "edit-config"
	OpCopyConfig         = "copy-config"
	OpDeleteConfig       = "delete-config"
	OpLock               = "lock"
	OpUnlock             = "unlock"
	OpCommit             = "commit"
	OpDiscardChanges     = "discard-changes"
	OpValidate           = "validate"
	OpCloseSession       = "close-session"
	OpKillSession        = "kill-session"
	OpCreateSubscription = "create-subscription"
)

const (
	StoreRunning   = "running"
	StoreCandidate = "candidate"
	StoreStartup   = "startup"
)

const (
	FilterSubtree = "subtree"
	FilterXPath   = "xpath"
)
