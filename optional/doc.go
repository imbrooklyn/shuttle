// Package optional provides an eager, typed representation of a value that may
// be absent.
//
// The zero value of Optional is absent. Presence is independent of the stored
// value, so zero values and typed nil values can be present. Optional values use
// ordinary shallow-copy semantics and are comparable whenever their element
// type is comparable.
package optional
