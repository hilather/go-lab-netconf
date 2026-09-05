package capabilities

// ID is a frozen capability identifier. Renames are a public-surface change.
type ID string

// Catalog IDs. Order matches catalog().
const (
	HealthLive         ID = "health.live"
	HealthReady        ID = "health.ready"
	SessionCreate      ID = "session.create"
	SessionGet         ID = "session.get"
	SessionDelete      ID = "session.delete"
	VersionGet         ID = "version.get"
	CapabilitiesGet    ID = "capabilities.get"
	StatusGet          ID = "status.get"
	SchemaGet          ID = "schema.get"
	FeaturesList       ID = "features.list"
	StateGet           ID = "state.get"
	StateValidate      ID = "state.validate"
	StateExport        ID = "state.export"
	StateReset         ID = "state.reset"
	ChangesPlan        ID = "changes.plan"
	ChangesApply       ID = "changes.apply"
	ProfilesList       ID = "profiles.list"
	ProfilesGet        ID = "profiles.get"
	UsersList          ID = "users.list"
	DatastoreGet       ID = "datastore.get"
	DatastoreSet       ID = "datastore.set"
	DatastoreCommit    ID = "datastore.commit"
	DatastoreDiscard   ID = "datastore.discard"
	SessionsList       ID = "sessions.list"
	SessionKill        ID = "session.kill"
	NotificationsList  ID = "notifications.list"
	NotificationsGet   ID = "notifications.get"
	NotificationsWait  ID = "notifications.wait"
	NotificationsClear ID = "notifications.clear"
	PreviewGet         ID = "preview.get"
	AuditQuery         ID = "audit.query"
)

// VersionTag is the capability schema version embedded on every row.
const VersionTag = "v1"
