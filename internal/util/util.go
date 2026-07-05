package util

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"
)

// GenerateRandomID generates a random ID string
func GenerateRandomID(prefix string, length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	
	return prefix + string(b)
}

// GenerateRandomTimeout generates a random timeout within a range
func GenerateRandomTimeout(min, max time.Duration) time.Duration {
	if min >= max {
		return min
	}
	
	return min + time.Duration(rand.Int63n(int64(max-min)))
}

// EncodeInt64 encodes an int64 to a byte slice
func EncodeInt64(n int64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(n))
	return buf
}

// DecodeInt64 decodes a byte slice to an int64
func DecodeInt64(buf []byte) int64 {
	return int64(binary.BigEndian.Uint64(buf))
}

// EncodeString encodes a string to a byte slice with length prefix
func EncodeString(s string) []byte {
	data := []byte(s)
	buf := make([]byte, 8+len(data))
	binary.BigEndian.PutUint64(buf[:8], uint64(len(data)))
	copy(buf[8:], data)
	return buf
}

// DecodeString decodes a byte slice to a string (with length prefix)
func DecodeString(buf []byte) (string, error) {
	if len(buf) < 8 {
		return "", fmt.Errorf("buffer too short")
	}
	
	length := binary.BigEndian.Uint64(buf[:8])
	if uint64(len(buf)) < 8+length {
		return "", fmt.Errorf("buffer too short for string")
	}
	
	return string(buf[8 : 8+length]), nil
}

// EncodeBytes encodes a byte slice with length prefix
func EncodeBytes(data []byte) []byte {
	buf := make([]byte, 8+len(data))
	binary.BigEndian.PutUint64(buf[:8], uint64(len(data)))
	copy(buf[8:], data)
	return buf
}

// DecodeBytes decodes a byte slice (with length prefix)
func DecodeBytes(buf []byte) ([]byte, error) {
	if len(buf) < 8 {
		return nil, fmt.Errorf("buffer too short")
	}
	
	length := binary.BigEndian.Uint64(buf[:8])
	if uint64(len(buf)) < 8+length {
		return nil, fmt.Errorf("buffer too short for data")
	}
	
	data := make([]byte, length)
	copy(data, buf[8:8+length])
	return data, nil
}

// Hash calculates a simple hash of a byte slice
func Hash(data []byte) uint64 {
	h := uint64(0)
	for _, b := range data {
		h = h*31 + uint64(b)
	}
	return h
}

// StringToBytes converts a string to bytes (helper for consistency)
func StringToBytes(s string) []byte {
	return []byte(s)
}

// BytesToString converts bytes to string (helper for consistency)
func BytesToString(b []byte) string {
	return string(b)
}

// HexEncode encodes bytes to hex string
func HexEncode(b []byte) string {
	return hex.EncodeToString(b)
}

// HexDecode decodes hex string to bytes
func HexDecode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

// Min returns the minimum of two integers
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max returns the maximum of two integers
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Contains checks if a slice contains a value
func Contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// RemoveString removes a string from a slice
func RemoveString(slice []string, value string) []string {
	for i, v := range slice {
		if v == value {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// UniqueStrings returns a slice with unique strings
func UniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	
	for _, v := range slice {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	
	return result
}

// Init initializes the random seed
func Init() {
	rand.Seed(time.Now().UnixNano())
}
