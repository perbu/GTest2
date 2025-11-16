package hpack

import (
	"bytes"
	"fmt"
	"io"
)

// Decoder decodes HPACK-encoded header blocks
type Decoder struct {
	table *Table
}

// NewDecoder creates a new HPACK decoder
func NewDecoder(maxDynamicTableSize uint32) *Decoder {
	return &Decoder{
		table: NewTable(maxDynamicTableSize),
	}
}

// Decode decodes an HPACK-encoded header block
func (d *Decoder) Decode(data []byte) ([]HeaderField, error) {
	buf := bytes.NewReader(data)
	var headers []HeaderField

	for buf.Len() > 0 {
		b, err := buf.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// Unread the byte so we can process it based on pattern
		buf.UnreadByte()

		var hf HeaderField

		switch {
		case b&0x80 != 0:
			// Indexed Header Field (1xxxxxxx)
			hf, err = d.decodeIndexed(buf)

		case b&0x40 != 0:
			// Literal with Incremental Indexing (01xxxxxx)
			hf, err = d.decodeLiteralWithIndexing(buf)

		case b&0x20 != 0:
			// Dynamic Table Size Update (001xxxxx)
			err = d.decodeDynamicTableSizeUpdate(buf)
			continue

		case b&0x10 != 0:
			// Literal Never Indexed (0001xxxx)
			hf, err = d.decodeLiteralNeverIndexed(buf)

		default:
			// Literal without Indexing (0000xxxx)
			hf, err = d.decodeLiteralWithoutIndexing(buf)
		}

		if err != nil {
			return nil, err
		}

		headers = append(headers, hf)
	}

	return headers, nil
}

// decodeIndexed decodes an indexed header field (1xxxxxxx)
func (d *Decoder) decodeIndexed(buf *bytes.Reader) (HeaderField, error) {
	index, err := decodeInteger(buf, 7)
	if err != nil {
		return HeaderField{}, err
	}

	if index == 0 {
		return HeaderField{}, fmt.Errorf("invalid index: 0")
	}

	hf, err := d.table.Lookup(int(index))
	if err != nil {
		return HeaderField{}, err
	}

	return hf, nil
}

// decodeLiteralWithIndexing decodes a literal with incremental indexing (01xxxxxx)
func (d *Decoder) decodeLiteralWithIndexing(buf *bytes.Reader) (HeaderField, error) {
	index, err := decodeInteger(buf, 6)
	if err != nil {
		return HeaderField{}, err
	}

	var name string
	if index == 0 {
		// New name
		name, err = decodeString(buf)
		if err != nil {
			return HeaderField{}, err
		}
	} else {
		// Indexed name
		hf, err := d.table.Lookup(int(index))
		if err != nil {
			return HeaderField{}, err
		}
		name = hf.Name
	}

	value, err := decodeString(buf)
	if err != nil {
		return HeaderField{}, err
	}

	hf := HeaderField{Name: name, Value: value}
	d.table.Add(hf)

	return hf, nil
}

// decodeLiteralNeverIndexed decodes a literal never indexed field (0001xxxx)
func (d *Decoder) decodeLiteralNeverIndexed(buf *bytes.Reader) (HeaderField, error) {
	index, err := decodeInteger(buf, 4)
	if err != nil {
		return HeaderField{}, err
	}

	var name string
	if index == 0 {
		// New name
		name, err = decodeString(buf)
		if err != nil {
			return HeaderField{}, err
		}
	} else {
		// Indexed name
		hf, err := d.table.Lookup(int(index))
		if err != nil {
			return HeaderField{}, err
		}
		name = hf.Name
	}

	value, err := decodeString(buf)
	if err != nil {
		return HeaderField{}, err
	}

	return HeaderField{Name: name, Value: value, Sensitive: true}, nil
}

// decodeLiteralWithoutIndexing decodes a literal without indexing (0000xxxx)
func (d *Decoder) decodeLiteralWithoutIndexing(buf *bytes.Reader) (HeaderField, error) {
	index, err := decodeInteger(buf, 4)
	if err != nil {
		return HeaderField{}, err
	}

	var name string
	if index == 0 {
		// New name
		name, err = decodeString(buf)
		if err != nil {
			return HeaderField{}, err
		}
	} else {
		// Indexed name
		hf, err := d.table.Lookup(int(index))
		if err != nil {
			return HeaderField{}, err
		}
		name = hf.Name
	}

	value, err := decodeString(buf)
	if err != nil {
		return HeaderField{}, err
	}

	return HeaderField{Name: name, Value: value}, nil
}

// decodeDynamicTableSizeUpdate decodes a dynamic table size update (001xxxxx)
func (d *Decoder) decodeDynamicTableSizeUpdate(buf *bytes.Reader) error {
	size, err := decodeInteger(buf, 5)
	if err != nil {
		return err
	}

	d.table.SetMaxDynamicSize(uint32(size))
	return nil
}

// decodeInteger decodes an integer with N-bit prefix as per RFC 7541 Section 5.1
func decodeInteger(buf *bytes.Reader, n uint) (uint64, error) {
	if n < 1 || n > 8 {
		return 0, fmt.Errorf("invalid prefix length: %d", n)
	}

	b, err := buf.ReadByte()
	if err != nil {
		return 0, err
	}

	max := uint64((1 << n) - 1)
	mask := byte(max)
	value := uint64(b & mask)

	if value < max {
		return value, nil
	}

	// Value uses continuation bytes
	m := uint64(0)
	for {
		b, err := buf.ReadByte()
		if err != nil {
			return 0, err
		}

		value += uint64(b&0x7f) << m
		m += 7

		if b&0x80 == 0 {
			break
		}

		if m > 63 {
			return 0, fmt.Errorf("integer overflow")
		}
	}

	return value, nil
}

// decodeString decodes a string as per RFC 7541 Section 5.2
func decodeString(buf *bytes.Reader) (string, error) {
	b, err := buf.ReadByte()
	if err != nil {
		return "", err
	}
	buf.UnreadByte()

	huffman := (b & 0x80) != 0

	length, err := decodeInteger(buf, 7)
	if err != nil {
		return "", err
	}

	data := make([]byte, length)
	n, err := io.ReadFull(buf, data)
	if err != nil {
		return "", err
	}
	if uint64(n) != length {
		return "", fmt.Errorf("incomplete string: expected %d bytes, got %d", length, n)
	}

	if huffman {
		// Huffman decoding not implemented yet - just return raw
		// For now, treat as literal
		return string(data), nil
	}

	return string(data), nil
}

// SetMaxDynamicTableSize updates the maximum dynamic table size
func (d *Decoder) SetMaxDynamicTableSize(size uint32) {
	d.table.SetMaxDynamicSize(size)
}
