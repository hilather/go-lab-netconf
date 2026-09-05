// Package restconf is the RFC 8040 JSON HTTP adapter over the datastore.
//
// It binds a dedicated HTTP listener (not the management mux). Writes
// call datastore.Handle.WriteRunningIfCandidateClean. The only media
// type is application/yang-data+json.
package restconf
