package network

import (
	"encoding/binary"
	"errors"
)

func Pack(p []byte, format string, data ...interface{}) (n int, err error) {
	var prefix, k int
	for _, c := range format {
		if c >= '0' && c <= '9' {
			prefix = prefix*10 + int(c-'0')
			continue
		}
		if prefix == 0 {
			prefix = 1
		}
		switch {
		case c == 'u' && prefix == 1:
			v := data[k].(uint32)
			binary.BigEndian.PutUint32(p, v)
			p = p[4:]
			n += 4
		case c == 'b' && prefix == 1:
			v := data[k].(uint8)
			p[0] = v
			p = p[1:]
			n += 1
		case c == 'b' && prefix > 1:
			v := data[k].([]uint8)
			copy(p, v[:prefix])
			p = p[prefix:]
			n += prefix
		case c == '_':
			p = p[prefix:]
			n += prefix
			k -= 1
		default:
			return n, errors.New("Unknown type")
		}
		k += 1
		prefix = 0
	}
	if k < len(data) {
		return n, errors.New("Not enough data arguments")
	}
	return
}

func Unpack(p []byte, format string, data ...interface{}) (n int, err error) {
	var prefix, k int
	for _, c := range format {
		if c >= '0' && c <= '9' {
			prefix = prefix*10 + int(c-'0')
			continue
		}
		if prefix == 0 {
			prefix = 1
		}
		switch {
		case c == 'u' && prefix == 1:
			v := data[k].(*uint32)
			*v = binary.BigEndian.Uint32(p)
			p = p[4:]
			n += 4
		case c == 'b' && prefix == 1:
			v := data[k].(*uint8)
			*v = p[0]
			p = p[1:]
			n += 1
		case c == 'b' && prefix > 1:
			v := data[k].([]uint8)
			copy(v, p[:prefix])
			p = p[prefix:]
			n += prefix
		case c == '_':
			p = p[prefix:]
			n += prefix
			k -= 1
		default:
			return n, errors.New("Unknown type")
		}
		k += 1
		prefix = 0
	}
	if k < len(data) {
		return n, errors.New("Not enough data arguments")
	}
	return
}
