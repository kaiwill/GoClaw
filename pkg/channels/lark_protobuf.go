package channels

import (
	"encoding/json"
	"fmt"
)

// parseVarint parses a Protobuf varint
func parseVarint(data []byte) (uint64, int) {
	var result uint64
	var shift uint
	for i := 0; i < len(data); i++ {
		b := data[i]
		result |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, i + 1
		}
		shift += 7
	}
	return result, len(data)
}

// parseProtobufField parses a Protobuf field header
func parseProtobufField(data []byte) (fieldNum int, wireType int, consumed int) {
	if len(data) == 0 {
		return 0, 0, 0
	}
	val, size := parseVarint(data)
	fieldNum = int(val >> 3)
	wireType = int(val & 0x7)
	return fieldNum, wireType, size
}

// parseProtobufBytes parses a length-delimited field
func parseProtobufBytes(data []byte) ([]byte, int) {
	if len(data) == 0 {
		return nil, 0
	}
	val, size := parseVarint(data)
	length := int(val)
	if len(data) < size+length {
		return nil, 0
	}
	return data[size : size+length], size + length
}

// parseProtobufToJSON parses Protobuf data and returns a JSON-compatible map
func parseProtobufToJSON(data []byte) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	remaining := data

	for len(remaining) > 0 {
		fieldNum, wireType, consumed := parseProtobufField(remaining)
		if consumed == 0 {
			break
		}
		remaining = remaining[consumed:]

		fieldName := fmt.Sprintf("field_%d", fieldNum)

		switch wireType {
		case 0: // Varint
			val, size := parseVarint(remaining)
			remaining = remaining[size:]
			result[fieldName] = val

		case 1: // 64-bit
			if len(remaining) >= 8 {
				result[fieldName] = "uint64_64bit"
				remaining = remaining[8:]
			} else {
				return nil, fmt.Errorf("not enough data for 64-bit value")
			}

		case 2: // Length-delimited
			val, size := parseProtobufBytes(remaining)
			if val != nil {
				remaining = remaining[size:]
				// Try to parse as string or JSON
				str := string(val)
				var jsonVal map[string]interface{}
				if err := json.Unmarshal(val, &jsonVal); err == nil {
					result[fieldName] = jsonVal
				} else {
					result[fieldName] = str
				}
			} else {
				return nil, fmt.Errorf("not enough data for length-delimited value")
			}

		default:
			return nil, fmt.Errorf("unknown wire type: %d", wireType)
		}
	}

	return result, nil
}
