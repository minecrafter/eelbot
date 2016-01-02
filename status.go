package main

import "bufio"
import "fmt"
import "net"
import "bytes"

const STATUS_RESPONSE = `{
	"version": {
		"name": "eelbot",
		"protocol": %d
	},
	"players": {
		"online": %d,
		"max": %d
	},	
	"description": {"text": "\u00A74\u00A7leelbot <3"}
}`

func handleStatus(conn net.Conn, reader *bufio.Reader, writer *bufio.Writer, protocol int32, max int) {
	// C->S 0x00 Status request
	id, err := readHeader(reader)
	if err != nil {
		conn.Close()
		return
	}
	if id != 0x00 {
		conn.Close()
		return
	}

	packetbuf := new(bytes.Buffer)

	// S->C Status response
	writeVarInt(packetbuf, 0x00)
	writeVarString(packetbuf, fmt.Sprintf(STATUS_RESPONSE, protocol, max, max))
	if err = writePacketBuf(writer, packetbuf); err != nil {
		conn.Close()
		return
	}

	// C->S 0x00 Ping
	id, err = readHeader(reader)
	if err != nil {
		conn.Close()
		return
	}
	if id != 0x01 {
		conn.Close()
		return
	}
	ping, err := readLong(reader)
	if err != nil {
		conn.Close()
		return
	}

	// S->C Ping
	writeVarInt(packetbuf, 0x01)
	writeLong(packetbuf, ping)
	if err = writePacketBuf(writer, packetbuf); err != nil {
		conn.Close()
		return
	}
}
