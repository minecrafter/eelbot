package main

import "fmt"
import "io"
import "encoding/binary"

func readHeader(reader io.Reader) (id int32, err error) {
	if _, err = readVarInt(reader); err != nil {
		return
	}
	return readVarInt(reader)
}

func readBool(reader io.Reader) (c bool, err error) {
	var value [1]byte
	_, err = reader.Read(value[0:])
	c = value[0] == 1
	return
}

func readByte(reader io.Reader) (c int8, err error) {
	err = binary.Read(reader, binary.BigEndian, &c)
	return
}

func readUnsignedByte(reader io.Reader) (c byte, err error) {
	err = binary.Read(reader, binary.BigEndian, &c)
	return
}

func readShort(reader io.Reader) (c int16, err error) {
	err = binary.Read(reader, binary.BigEndian, &c)
	return
}

func readInt(reader io.Reader) (c int32, err error) {
	err = binary.Read(reader, binary.BigEndian, &c)
	return
}

func readLong(reader io.Reader) (c int64, err error) {
	err = binary.Read(reader, binary.BigEndian, &c)
	return
}

func readFloat(reader io.Reader) (c float32, err error) {
	err = binary.Read(reader, binary.BigEndian, &c)
	return
}

func readDouble(reader io.Reader) (c float64, err error) {
	err = binary.Read(reader, binary.BigEndian, &c)
	return
}

func readVarInt(reader io.Reader) (int32, error) {
	var x uint64
	var s uint
	var b byte
	for i := 0; i <= 10; i++ {
		if err := binary.Read(reader, binary.BigEndian, &b); err != nil {
			return 0, err
		}
		if b < 0x80 {
			if i > 9 || i == 9 && b > 1 {
				return 0, fmt.Errorf("Varint overflow")
			}
			return int32(uint32(x | uint64(b)<<s)), nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
	return 0, fmt.Errorf("Varint overflow")
}

func readVarString(reader io.Reader, maxLength int) (c string, err error) {
	length, err := readVarInt(reader)
	if err != nil {
		return
	}
	if length < 0 {
		return "", fmt.Errorf("String length < 0")
	}
	if int(length) > maxLength {
		return "", fmt.Errorf("String length > maxLength")
	}
	b := make([]byte, length)
	if _, err = io.ReadFull(reader, b); err != nil {
		return
	}
	c = string(b)
	return
}
