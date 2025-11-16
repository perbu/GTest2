package hpack

import (
	"fmt"
)

// HeaderField represents a header name-value pair
type HeaderField struct {
	Name      string
	Value     string
	Sensitive bool // Never index this field
}

// Size returns the size of the header field as defined in RFC 7541 Section 4.1
// size = len(name) + len(value) + 32
func (hf HeaderField) Size() uint32 {
	return uint32(len(hf.Name) + len(hf.Value) + 32)
}

// staticTable is the static table defined in RFC 7541 Appendix A
var staticTable = []HeaderField{
	{":authority", "", false},                      // 1
	{":method", "GET", false},                      // 2
	{":method", "POST", false},                     // 3
	{":path", "/", false},                          // 4
	{":path", "/index.html", false},                // 5
	{":scheme", "http", false},                     // 6
	{":scheme", "https", false},                    // 7
	{":status", "200", false},                      // 8
	{":status", "204", false},                      // 9
	{":status", "206", false},                      // 10
	{":status", "304", false},                      // 11
	{":status", "400", false},                      // 12
	{":status", "404", false},                      // 13
	{":status", "500", false},                      // 14
	{"accept-charset", "", false},                  // 15
	{"accept-encoding", "gzip, deflate", false},    // 16
	{"accept-language", "", false},                 // 17
	{"accept-ranges", "", false},                   // 18
	{"accept", "", false},                          // 19
	{"access-control-allow-origin", "", false},     // 20
	{"age", "", false},                             // 21
	{"allow", "", false},                           // 22
	{"authorization", "", false},                   // 23
	{"cache-control", "", false},                   // 24
	{"content-disposition", "", false},             // 25
	{"content-encoding", "", false},                // 26
	{"content-language", "", false},                // 27
	{"content-length", "", false},                  // 28
	{"content-location", "", false},                // 29
	{"content-range", "", false},                   // 30
	{"content-type", "", false},                    // 31
	{"cookie", "", false},                          // 32
	{"date", "", false},                            // 33
	{"etag", "", false},                            // 34
	{"expect", "", false},                          // 35
	{"expires", "", false},                         // 36
	{"from", "", false},                            // 37
	{"host", "", false},                            // 38
	{"if-match", "", false},                        // 39
	{"if-modified-since", "", false},               // 40
	{"if-none-match", "", false},                   // 41
	{"if-range", "", false},                        // 42
	{"if-unmodified-since", "", false},             // 43
	{"last-modified", "", false},                   // 44
	{"link", "", false},                            // 45
	{"location", "", false},                        // 46
	{"max-forwards", "", false},                    // 47
	{"proxy-authenticate", "", false},              // 48
	{"proxy-authorization", "", false},             // 49
	{"range", "", false},                           // 50
	{"referer", "", false},                         // 51
	{"refresh", "", false},                         // 52
	{"retry-after", "", false},                     // 53
	{"server", "", false},                          // 54
	{"set-cookie", "", false},                      // 55
	{"strict-transport-security", "", false},       // 56
	{"transfer-encoding", "", false},               // 57
	{"user-agent", "", false},                      // 58
	{"vary", "", false},                            // 59
	{"via", "", false},                             // 60
	{"www-authenticate", "", false},                // 61
}

const staticTableSize = 61

// DynamicTable maintains the dynamic table for HPACK encoding/decoding
type DynamicTable struct {
	entries    []HeaderField
	size       uint32 // Current size in bytes
	maxSize    uint32 // Maximum size in bytes
}

// NewDynamicTable creates a new dynamic table with the given maximum size
func NewDynamicTable(maxSize uint32) *DynamicTable {
	return &DynamicTable{
		entries: make([]HeaderField, 0),
		size:    0,
		maxSize: maxSize,
	}
}

// SetMaxSize updates the maximum size and evicts entries if necessary
func (dt *DynamicTable) SetMaxSize(maxSize uint32) {
	dt.maxSize = maxSize
	dt.evict()
}

// Add adds a header field to the dynamic table
func (dt *DynamicTable) Add(hf HeaderField) {
	// Insert at the beginning (newest entries have lowest indices)
	dt.entries = append([]HeaderField{hf}, dt.entries...)
	dt.size += hf.Size()
	dt.evict()
}

// evict removes entries from the end until the table size is within maxSize
func (dt *DynamicTable) evict() {
	for dt.size > dt.maxSize && len(dt.entries) > 0 {
		// Remove from the end (oldest entries)
		lastIdx := len(dt.entries) - 1
		dt.size -= dt.entries[lastIdx].Size()
		dt.entries = dt.entries[:lastIdx]
	}
}

// Len returns the number of entries in the dynamic table
func (dt *DynamicTable) Len() int {
	return len(dt.entries)
}

// Get retrieves an entry from the dynamic table by index (1-based, relative to dynamic table start)
func (dt *DynamicTable) Get(index int) (HeaderField, bool) {
	if index < 1 || index > len(dt.entries) {
		return HeaderField{}, false
	}
	return dt.entries[index-1], true
}

// Search looks for a header field in the dynamic table
// Returns (index, nameMatch, valueMatch)
func (dt *DynamicTable) Search(name, value string) (int, bool, bool) {
	for i, hf := range dt.entries {
		if hf.Name == name {
			if hf.Value == value {
				return i + 1, true, true // Full match
			}
			// Name matches but not value - continue looking for full match
		}
	}

	// Look again for just name match
	for i, hf := range dt.entries {
		if hf.Name == name {
			return i + 1, true, false
		}
	}

	return 0, false, false
}

// Table combines static and dynamic tables for HPACK lookups
type Table struct {
	dynamic *DynamicTable
}

// NewTable creates a new combined table
func NewTable(maxDynamicSize uint32) *Table {
	return &Table{
		dynamic: NewDynamicTable(maxDynamicSize),
	}
}

// Lookup retrieves a header field by absolute index
// Indices 1-61 are static table, 62+ are dynamic table
func (t *Table) Lookup(index int) (HeaderField, error) {
	if index < 1 {
		return HeaderField{}, fmt.Errorf("invalid index: %d (must be >= 1)", index)
	}

	if index <= staticTableSize {
		return staticTable[index-1], nil
	}

	// Dynamic table index
	dynamicIndex := index - staticTableSize
	if hf, ok := t.dynamic.Get(dynamicIndex); ok {
		return hf, nil
	}

	return HeaderField{}, fmt.Errorf("index %d not found in dynamic table", index)
}

// Search looks for a header field in both tables
// Returns (absoluteIndex, nameMatch, valueMatch)
func (t *Table) Search(name, value string) (int, bool, bool) {
	// Search static table first
	for i, hf := range staticTable {
		if hf.Name == name {
			if hf.Value == value {
				return i + 1, true, true // Full match
			}
		}
	}

	// Search dynamic table
	if dynIdx, nameMatch, valueMatch := t.dynamic.Search(name, value); nameMatch {
		absoluteIndex := dynIdx + staticTableSize
		return absoluteIndex, nameMatch, valueMatch
	}

	// Look for name-only match in static table
	for i, hf := range staticTable {
		if hf.Name == name {
			return i + 1, true, false
		}
	}

	return 0, false, false
}

// Add adds a header field to the dynamic table
func (t *Table) Add(hf HeaderField) {
	t.dynamic.Add(hf)
}

// SetMaxDynamicSize updates the maximum dynamic table size
func (t *Table) SetMaxDynamicSize(maxSize uint32) {
	t.dynamic.SetMaxSize(maxSize)
}

// DynamicTableSize returns the current dynamic table size in bytes
func (t *Table) DynamicTableSize() uint32 {
	return t.dynamic.size
}

// DynamicTableLen returns the number of entries in the dynamic table
func (t *Table) DynamicTableLen() int {
	return t.dynamic.Len()
}
