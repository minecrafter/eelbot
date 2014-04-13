/*
   Copyright 2014 TeapotDev

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package main

import "flag"
import "net"
import "fmt"
import "bufio"
import "bytes"
import "encoding/hex"
import "crypto/rand"
import "time"
import "os"
import "io"
import "sync"

func main() {
	fmt.Println("*** eelbot by TeapotDev (Minecraft 1.7.2-1.7.5) ***")
	proxy := flag.String("proxy", "127.0.0.1:25588", "proxy address")
	target := flag.String("target", "127.0.0.1:25565", "server address")
	count := flag.Int("count", 10, "count of bots")
	joindelay := flag.Int("joind", 0, "timeout between bot joins in millis")
	errdelay := flag.Int("errd", 4100, "timeout in millis if client was kicked while connecting")
	eeldelay := flag.Int("eeld", 100, "eel timeout (snake)")
	flag.Parse()

	listener, err := net.Listen("tcp", *proxy)
	if err != nil {
		fmt.Println("## Error starting proxy:", err.Error())
		os.Exit(1)
	}

	var conn net.Conn
	var reader *bufio.Reader
	var writer *bufio.Writer
	var protocolVersion int32
	packetbuf := new(bytes.Buffer)
	fmt.Println("## Waiting for connection to proxy...")
	for {
		conn, err = listener.Accept()
		if err != nil {
			fmt.Println("# Error accepting client:", err.Error())
			conn.Close()
			continue
		}
		reader = bufio.NewReader(conn)
		writer = bufio.NewWriter(conn)

		// C->S 0x00 Handshake
		id, err := readHeader(reader)
		if err != nil {
			fmt.Println("# Error receiving from client:", err.Error())
			conn.Close()
			continue
		}
		if id != 0x00 {
			fmt.Println("# Error receiving from client: unexpected packet")
			conn.Close()
			continue
		}
		protocolVersion, _ = readVarInt(reader)
		readVarString(reader, 64)
		readShort(reader)
		if next, err := readVarInt(reader); next != 2 {
			if err != nil {
				fmt.Println("# Error receiving from client:", err.Error())
				conn.Close()
				continue
			}
			fmt.Println("# Status query!")
			go handleStatus(conn, reader, writer, protocolVersion, *count)
			continue
		}

		// C->S Login start
		id, err = readHeader(reader)
		if err != nil {
			fmt.Println("# Error receiving from client:", err.Error())
			conn.Close()
			continue
		}
		if id != 0x00 {
			fmt.Println("# Error receiving from client: unexpected packet")
			conn.Close()
			continue
		}
		readVarString(reader, 64)
		break
	}

	// S->C Login success
	writeVarInt(packetbuf, 0x02)
	writeVarString(packetbuf, "eel")
	writeVarString(packetbuf, "eel")
	if err = writePacketBuf(writer, packetbuf); err != nil {
		fmt.Println("## Error sending to client: " + err.Error())
		os.Exit(1)
	}

	keepAliveStop := make(chan int)
	keepAliveStopped := make(chan int)
	go keepAlive(writer, keepAliveStop, keepAliveStopped, true)

	var firstReader *bufio.Reader // one of bots is client (main bot)
	otherWriters := make([]*bufio.Writer, *count)

	fmt.Println("## Client connected to proxy! Connecting eel to target...")
	for i := 0; i < *count; i++ {
		nick := randomNick()
		fmt.Printf("# Connecting %d: %s\n", i+1, nick)
		other, err := net.Dial("tcp", *target)
		if err != nil {
			fmt.Printf("# Error connecting %d: %s\n", i+1, err.Error())
			time.Sleep(time.Duration(*errdelay) * time.Millisecond)
			i--
			continue
		}
		otherWriter := bufio.NewWriter(other)

		// C->S Handshake
		writeVarInt(packetbuf, 0x00)
		writeVarInt(packetbuf, protocolVersion)
		writeVarString(packetbuf, "minecraft.net")
		writeShort(packetbuf, 25565)
		writeVarInt(packetbuf, 2)
		if err = writePacketBuf(otherWriter, packetbuf); err != nil {
			fmt.Printf("# Error connecting %d: %s\n", i+1, err.Error())
			time.Sleep(time.Duration(*errdelay) * time.Millisecond)
			other.Close()
			i--
			continue
		}

		// C->S Login start
		writeVarInt(packetbuf, 0x00)
		writeVarString(packetbuf, nick)
		if err = writePacketBuf(otherWriter, packetbuf); err != nil {
			fmt.Printf("# Error connecting %d: %s\n", i+1, err.Error())
			time.Sleep(time.Duration(*errdelay) * time.Millisecond)
			other.Close()
			i--
			continue
		}

		otherReader := bufio.NewReader(other)

		// S->C Login success
		id, err := readHeader(otherReader)
		if err != nil {
			fmt.Printf("# Error connecting %d: %s\n", i+1, err.Error())
			time.Sleep(time.Duration(*errdelay) * time.Millisecond)
			other.Close()
			i--
			continue
		}
		if id != 0x02 {
			fmt.Printf("# Error connecting %d: unexpected packet\n", i+1)
			time.Sleep(time.Duration(*errdelay) * time.Millisecond)
			other.Close()
			i--
			continue
		}
		readVarString(otherReader, 64)
		_, err = readVarString(otherReader, 64)
		if err != nil {
			fmt.Printf("# Error connecting %d: %s\n", i+1, err.Error())
			time.Sleep(time.Duration(*errdelay) * time.Millisecond)
			other.Close()
			i--
			continue
		}

		if firstReader == nil {
			firstReader = otherReader
		}
		otherWriters[i] = otherWriter
		go keepAlive(otherWriter, keepAliveStop, keepAliveStopped, false)
		time.Sleep(time.Duration(*joindelay) * time.Millisecond)
	}

	firstWriter := otherWriters[0] // main bot
	otherWriters = otherWriters[1:]

	mutexes := make([]*sync.Mutex, len(otherWriters))
	for i := 0; i < len(mutexes); i++ {
		mutexes[i] = new(sync.Mutex)
	}

	// stopping async keep alive
	for i := 0; i < *count+1; i++ { // count of bots + client connected to proxy
		keepAliveStop <- 1
	}
	for i := 0; i < *count+1; i++ {
		<-keepAliveStopped
	}

	go func() {
		var buffer [2048]byte
		for {
			// read from main bot stream and write to client
			count, err := firstReader.Read(buffer[:])
			if err != nil {
				fmt.Println("# Error reading from main connection:", err.Error())
				os.Exit(1)
			}
			_, err = writer.Write(buffer[:count])
			if err != nil {
				fmt.Println("# Error writing to main connection:", err.Error())
				os.Exit(1)
			}
			err = writer.Flush()
			if err != nil {
				fmt.Println("# Error writing to main connection:", err.Error())
				os.Exit(1)
			}
		}
	}()

	eelDuration := time.Duration(*eeldelay) * time.Millisecond
	fmt.Println("## Redirecting!")
	for {
		// reading packet from client
		length, err := readVarInt(reader)
		if err != nil {
			fmt.Println("# Error reading from main connection:", err.Error())
			os.Exit(1)
		}
		buffer := make([]byte, length)
		_, err = io.ReadFull(reader, buffer)
		if err != nil {
			fmt.Println("# Error reading from main connection:", err.Error())
			os.Exit(1)
		}

		// writing packet to main bot
		writeVarInt(firstWriter, length)
		firstWriter.Write(buffer)
		firstWriter.Flush()

		go func() {
			// writing packet to other bots with timeout
			for i, otherWriter := range otherWriters {
				time.Sleep(eelDuration)
				mutexes[i].Lock()
				writeVarInt(otherWriter, length)
				otherWriter.Write(buffer)
				otherWriter.Flush()
				mutexes[i].Unlock()
			}
		}()
	}
}

func randomNick() string {
	buf := make([]byte, 8)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err.Error())
	}
	return hex.EncodeToString(buf)
}

func keepAlive(writer *bufio.Writer, stop, stopped chan int, quitOnErr bool) { // used to keep alive while connecting other bots
	ticker := time.NewTicker(time.Second)
	defer func() {
		ticker.Stop()
		stopped <- 1
	}()
	for i := int32(1); ; i++ {
		select {
		case <-ticker.C:
		case <-stop:
			return
		}
		// C->S 0x00 Keep alive
		writeByte(writer, 5) // packet length
		writeByte(writer, 0) // packet id
		writeInt(writer, i)  // keep alive id
		if err := writer.Flush(); quitOnErr && err != nil {
			fmt.Println("## Error while sending keep alive: " + err.Error())
			os.Exit(1)
		}
	}
}
