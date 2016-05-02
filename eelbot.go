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

const PROTOCOL18 = 47

func main() {
	fmt.Println("*** eelbot by TeapotDev (Minecraft 1.7.2-1.8.x) ***")
	proxy := flag.String("proxy", "127.0.0.1:25588", "proxy address (client is connecting to it)")
	target := flag.String("target", "127.0.0.1:25565", "target server address")
	count := flag.Int("count", 10, "amount of bots to be connected")
	joindelay := flag.Int("joind", 0, "timeout between bot joins in milliseconds")
	errdelay := flag.Int("errd", 4100, "timeout in milliseconds if client was kicked while connecting")
	eeldelay := flag.Int("eeld", 100, "timeout between bots' actions (snake effect)")
	keepConn := flag.Bool("keep", false, "keeps connections after main client disconnects")
	ver18 := flag.Bool("ver18", false, "use 1.8+ version (packet compression)")
	flag.Parse()

	protocol := 0
	if *ver18 {
		protocol = PROTOCOL18
	}

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
	writeVarString(packetbuf, "069a79f4-44e9-4726-a5be-fca90e38aaf5")
	writeVarString(packetbuf, "eel")
	if err = writePacketBuf(writer, packetbuf); err != nil {
		fmt.Println("## Error sending to client: " + err.Error())
		os.Exit(1)
	}

	keepAliveStop := make(chan int)
	keepAliveStopped := make(chan int)
	go keepAlive(writer, keepAliveStop, keepAliveStopped, true, true, protocol, nil)

	var firstReader *bufio.Reader // one of bots is client (main bot)
	otherWriters := make([]*bufio.Writer, *count)

	mutexes := make([]*sync.Mutex, len(otherWriters))
	for i := 0; i < len(mutexes); i++ {
		mutexes[i] = new(sync.Mutex)
	}

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
			if id == 0x00 { // disconnect
				reason, err := readVarString(otherReader, 512)
				if err == nil {
					fmt.Printf("# Error connecting %d: kicked: %s\n", i+1, reason)
				} else {
					fmt.Printf("# Error connecting %d: %s\n", i+1, err.Error())
				}
			} else if id == 0x01 { // encryption request
				fmt.Printf("# Error connecting %d: online mode not supported\n", i+1)
			} else {
				fmt.Printf("# Error connecting %d: unexpected packet\n", i+1)
			}
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
		} else {
			go dummyRead(otherReader)
		}
		otherWriters[i] = otherWriter
		go keepAlive(otherWriter, nil, keepAliveStopped, false, false, protocol, mutexes[i])
		time.Sleep(time.Duration(*joindelay) * time.Millisecond)
	}

	firstWriter := otherWriters[0] // main bot
	otherWriters = otherWriters[1:]

	// stopping async keep alive
	keepAliveStop <- 1
	<-keepAliveStopped

	go func() {
		var buffer [2048]byte
		for {
			// read from main bot stream and write to client
			count, err := firstReader.Read(buffer[:])
			if err != nil {
				fmt.Println("# Error reading from main connection:", err.Error())
				break
			}
			_, err = writer.Write(buffer[:count])
			if err != nil {
				fmt.Println("# Error writing to main connection:", err.Error())
				break
			}
			err = writer.Flush()
			if err != nil {
				fmt.Println("# Error writing to main connection:", err.Error())
				break
			}
		}

		if !(*keepConn) {
			os.Exit(0)
		}
	}()

	eelDuration := time.Duration(*eeldelay) * time.Millisecond
	fmt.Println("## Redirecting!")
	for {
		// reading packet from client
		length, err := readVarInt(reader)
		if err != nil {
			fmt.Println("# Error reading from main connection:", err.Error())
			break
		}
		buffer := make([]byte, length)
		_, err = io.ReadFull(reader, buffer)
		if err != nil {
			fmt.Println("# Error reading from main connection:", err.Error())
			break
		}

		// writing packet to main bot
		mutexes[0].Lock()
		writeVarInt(firstWriter, length)
		firstWriter.Write(buffer)
		firstWriter.Flush()
		mutexes[0].Unlock()

		// writing packet to other bots with timeout
		if *eeldelay > 0 {
			go func() {
				for i, otherWriter := range otherWriters {
					time.Sleep(eelDuration)
					mutexes[i+1].Lock()
					writeVarInt(otherWriter, length)
					otherWriter.Write(buffer)
					otherWriter.Flush()
					mutexes[i+1].Unlock()
				}
			}()
		} else {
			for _, otherWriter := range otherWriters {
				writeVarInt(otherWriter, length)
				otherWriter.Write(buffer)
				otherWriter.Flush()
			}
		}
	}

	if *keepConn {
		for j := int32(0); ; j++ {
			time.Sleep(5 * time.Second)

			writeKeepAlive(firstWriter, false, protocol)
			firstWriter.Flush()

			for i, otherWriter := range otherWriters {
				mutexes[i].Lock()
				writeKeepAlive(otherWriter, false, protocol)
				otherWriter.Flush()
				mutexes[i].Unlock()
			}
		}
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

func keepAlive(writer *bufio.Writer, stop, stopped chan int, quitOnErr, toClient bool, protocol int, mutex *sync.Mutex) { // used to keep alive while connecting other bots
	ticker := time.NewTicker(5 * time.Second)
	defer func() {
		ticker.Stop()
		if stopped != nil {
			stopped <- 1
		}
	}()
	for {
		select {
		case <-ticker.C:
		case <-stop:
			return
		}

		if mutex != nil {
			mutex.Lock()
		}

		writeKeepAlive(writer, toClient, protocol)
		if err := writer.Flush(); quitOnErr && err != nil {
			fmt.Println("## Error while sending keep alive: " + err.Error())
			if mutex != nil {
				mutex.Unlock()
			}
			return
		}

		if mutex != nil {
			mutex.Unlock()
		}
	}
}

func writeKeepAlive(writer *bufio.Writer, toClient bool, protocol int) {
	if !toClient && protocol >= 47 {
		writeByte(writer, 3) // packet length
		writeByte(writer, 0) // compressed length
	} else {
		writeByte(writer, 5) // packet length
	}
	writeByte(writer, 0) // packet id
	if protocol >= 47 {
		writeByte(writer, 0) // keep alive id
	} else {
		writeInt(writer, 0) // keep alive id
	}
}

func dummyRead(reader io.Reader) {
	var buf [4096]byte
	for {
		_, err := reader.Read(buf[:])
		if err != nil {
			return
		}
	}
}
