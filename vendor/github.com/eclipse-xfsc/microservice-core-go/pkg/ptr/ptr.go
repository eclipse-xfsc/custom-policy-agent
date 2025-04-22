/*
Package ptr contains utility functions for converting values to pointer values.
*/
package ptr

import "time"

// Bool returns a pointer to a bool value.
func Bool(b bool) *bool {
	return &b
}

// Byte returns a pointer to a byte value.
func Byte(b byte) *byte {
	return &b
}

// Int returns a pointer to an int value.
func Int(i int) *int {
	return &i
}

// Int8 returns a pointer to an int8 value.
func Int8(i int8) *int8 {
	return &i
}

// Int16 returns a pointer to an int16 value.
func Int16(i int16) *int16 {
	return &i
}

// Int32 returns a pointer to an int32 value.
func Int32(i int32) *int32 {
	return &i
}

// Int64 returns a pointer to an int64 value.
func Int64(i int64) *int64 {
	return &i
}

// Uint returns a pointer to a uint value.
func Uint(u uint) *uint {
	return &u
}

// Uint8 returns a pointer to a uint8 value.
func Uint8(u uint8) *uint8 {
	return &u
}

// Uint16 returns a pointer to a uint16 value.
func Uint16(u uint16) *uint16 {
	return &u
}

// Uint32 returns a pointer to a uint32 value.
func Uint32(u uint32) *uint32 {
	return &u
}

// Uint64 returns a pointer to a uint64 value.
func Uint64(u uint64) *uint64 {
	return &u
}

// Float32 returns a pointer to a float32 value.
func Float32(f float32) *float32 {
	return &f
}

// Float64 returns a pointer to a float64 value.
func Float64(f float64) *float64 {
	return &f
}

// Rune returns a pointer to a rune value.
func Rune(r rune) *rune {
	return &r
}

// String returns a pointer to a string value.
func String(s string) *string {
	return &s
}

// Time returns a pointer to a time.Time value.
func Time(t time.Time) *time.Time {
	return &t
}
