package main

import "io"
import "bufio"
import "encoding/binary"
import "bytes"

func writePacketBuf(writer *bufio.Writer, buf *bytes.Buffer) (err error) {
	defer buf.Reset()
	b := buf.Bytes()
	if err = writeVarInt(writer, int32(len(b))); err != nil {
		return
	}
	if _, err = writer.Write(b); err != nil {
		return
	}
	err = writer.Flush()
	return
}

func writeByte(writer io.Writer, c int8) error {
	return binary.Write(writer, binary.BigEndian, &c)
}

func writeUnsignedByte(writer io.Writer, c byte) error {
	return binary.Write(writer, binary.BigEndian, &c)
}

func writeShort(writer io.Writer, c int16) error {
	return binary.Write(writer, binary.BigEndian, &c)
}

func writeInt(writer io.Writer, c int32) error {
	return binary.Write(writer, binary.BigEndian, &c)
}

func writeLong(writer io.Writer, c int64) error {
	return binary.Write(writer, binary.BigEndian, &c)
}

func writeFloat(writer io.Writer, c float32) error {
	return binary.Write(writer, binary.BigEndian, &c)
}

func writeDouble(writer io.Writer, c float64) error {
	return binary.Write(writer, binary.BigEndian, &c)
}

func writeVarInt(writer io.Writer, c int32) (err error) {
	for c >= 0x80 {
		err = writeUnsignedByte(writer, byte(c)|0x80)
		if err != nil {
			return
		}
		c >>= 7
	}
	writeUnsignedByte(writer, byte(c))
	return
}

func writeVarString(writer io.Writer, c string) (err error) {
	err = writeVarInt(writer, int32(len(c)))
	if err != nil {
		return
	}
	_, err = writer.Write([]byte(c))
	return
}
