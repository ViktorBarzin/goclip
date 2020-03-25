package common

import "sort"

const (
	DefaultMulticastAddress = "239.0.0.0:9999"
	DefaultRunTimeout       = 60
	MaxDatagramSize         = 8192
	PeerToPeerListenPort    = 30099
	AllInterfaces           = "all"
)

// Contains check if a string is contained in the []string
func Contains(s []string, searchterm string) bool {
	i := sort.SearchStrings(s, searchterm)
	return i < len(s) && s[i] == searchterm
}

// Remove a string from a []string and return the modified slice
func Remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}
