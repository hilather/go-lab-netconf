// Package datastore is the per-profile-instance running/candidate/startup store.
//
// Each Handle owns its own triple and lock table. Locks are not
// process-global. RESTCONF writes running through
// WriteRunningIfCandidateClean; NETCONF edit-config uses Edit on
// candidate and Commit.
package datastore
