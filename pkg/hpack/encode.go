package hpack

import (
	"bytes"
	"fmt"
)

// Encoder encodes header fields using HPACK
type Encoder struct {
	table *Table
	buf   bytes.Buffer
}

// NewEncoder creates a new HPACK encoder
func NewEncoder(maxDynamicTableSize uint32) *Encoder {
	return &Encoder{
		table: NewTable(maxDynamicTableSize),
	}
}

// Encode encodes a list of header fields into HPACK format
func (e *Encoder) Encode(headers []HeaderField) ([]byte, error) {
	e.buf.Reset()

	for _, hf := range headers {
		if err := e.encodeField(hf); err != nil {
			return nil, err
		}
	}

	return e.buf.Bytes(), nil
}

// encodeField encodes a single header field
func (e *Encoder) encodeField(hf HeaderField) error {
	// Search for the field in the table
	index, nameMatch, valueMatch := e.table.Search(hf.Name, hf.Value)

	if valueMatch {
		// Indexed Header Field Representation
		return e.encodeIndexed(index)
	} else if nameMatch {
		// Literal with Incremental Indexing — Indexed Name
		if hf.Sensitive {
			return e.encodeLiteralNeverIndexed(index, hf.Value)
		}
		return e.encodeLiteralWithIndexing(index, hf.Value)
	} else {
		// Literal with Incremental Indexing — New Name
		if hf.Sensitive {
			return e.encodeLiteralNeverIndexedNewName(hf.Name, hf.Value)
		}
		return e.encodeLiteralWithIndexingNewName(hf.Name, hf.Value)
	}
}

// encodeIndexed encodes an indexed header field (pattern: 1xxxxxxx)
func (e *Encoder) encodeIndexed(index int) error {
	return encodeInteger(&e.buf, 7, 0x80, uint64(index))
}

// encodeLiteralWithIndexing encodes a literal with incremental indexing - indexed name
// Pattern: 01xxxxxx
func (e *Encoder) encodeLiteralWithIndexing(nameIndex int, value string) error {
	if err := encodeInteger(&e.buf, 6, 0x40, uint64(nameIndex)); err != nil {
		return err
	}
	if err := encodeString(&e.buf, value, false); err != nil {
		return err
	}

	// Add to dynamic table
	if nameIndex > 0 {
		if hf, err := e.table.Lookup(nameIndex); err == nil {
			e.table.Add(HeaderField{Name: hf.Name, Value: value})
		}
	}

	return nil
}

// encodeLiteralWithIndexingNewName encodes a literal with incremental indexing - new name
// Pattern: 01000000
func (e *Encoder) encodeLiteralWithIndexingNewName(name, value string) error {
	e.buf.WriteByte(0x40) // 01000000
	if err := encodeString(&e.buf, name, false); err != nil {
		return err
	}
	if err := encodeString(&e.buf, value, false); err != nil {
		return err
	}

	// Add to dynamic table
	e.table.Add(HeaderField{Name: name, Value: value})

	return nil
}

// encodeLiteralNeverIndexed encodes a literal that should never be indexed - indexed name
// Pattern: 0001xxxx
func (e *Encoder) encodeLiteralNeverIndexed(nameIndex int, value string) error {
	if err := encodeInteger(&e.buf, 4, 0x10, uint64(nameIndex)); err != nil {
		return err
	}
	return encodeString(&e.buf, value, false)
}

// encodeLiteralNeverIndexedNewName encodes a literal that should never be indexed - new name
// Pattern: 00010000
func (e *Encoder) encodeLiteralNeverIndexedNewName(name, value string) error {
	e.buf.WriteByte(0x10) // 00010000
	if err := encodeString(&e.buf, name, false); err != nil {
		return err
	}
	return encodeString(&e.buf, value, false)
}

// SetMaxDynamicTableSize updates the maximum dynamic table size
func (e *Encoder) SetMaxDynamicTableSize(size uint32) error {
	// Encode dynamic table size update (pattern: 001xxxxx)
	if err := encodeInteger(&e.buf, 5, 0x20, uint64(size)); err != nil {
		return err
	}
	e.table.SetMaxDynamicSize(size)
	return nil
}

// encodeInteger encodes an integer with N-bit prefix as per RFC 7541 Section 5.1
func encodeInteger(buf *bytes.Buffer, n uint, prefix byte, value uint64) error {
	if n < 1 || n > 8 {
		return fmt.Errorf("invalid prefix length: %d", n)
	}

	max := uint64((1 << n) - 1)

	if value < max {
		// Value fits in the N-bit prefix
		buf.WriteByte(prefix | byte(value))
		return nil
	}

	// Value doesn't fit, use continuation
	buf.WriteByte(prefix | byte(max))
	value -= max

	for value >= 128 {
		buf.WriteByte(byte((value & 0x7f) | 0x80))
		value >>= 7
	}
	buf.WriteByte(byte(value))

	return nil
}

// encodeString encodes a string as per RFC 7541 Section 5.2
func encodeString(buf *bytes.Buffer, s string, huffman bool) error {
	if huffman {
		// Huffman encoding not implemented yet - just use literal
		// For now, always use literal encoding
		huffman = false
	}

	prefix := byte(0)
	if huffman {
		prefix = 0x80
	}

	data := []byte(s)
	if err := encodeInteger(buf, 7, prefix, uint64(len(data))); err != nil {
		return err
	}
	buf.Write(data)
	return nil
}
