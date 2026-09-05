// Package audit records mutation events on a bounded in-process ring.
//
// Hook delivery failure is counted and never fail-closes the mutation.
package audit
