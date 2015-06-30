package plugin

import (
	"fmt"
	"os"
	"sync"
)

const (
	Noop     = iota
	Oneway   = iota
	Request  = iota
	Response = iota
)

var nextID = 0

var LogPrefix = ""

func UniqueID() int {
	nextID++
	return nextID
}

type Connection struct {
	readLock  sync.Mutex
	writeLock sync.Mutex
	id        int
	inFile    *os.File
	outFile   *os.File

	channelsLock     sync.Mutex
	responseChannels map[int]chan *Chunk

	reqFn func(*Connection, *Chunk) *Chunk
}

type Chunk struct {
	threadID  int
	direction int // Oneway vs Request vs Response
	cmd       string
	buffer    []byte
}

func (c *Connection) Setup(in int, out int, reqFn func(*Connection, *Chunk) *Chunk) {
	c.id = UniqueID()
	c.inFile = os.NewFile(uintptr(in), "in")
	c.outFile = os.NewFile(uintptr(out), "out")
	c.reqFn = reqFn
	c.responseChannels = map[int]chan *Chunk{}

	go c.IncomingProcessor()
}

func (c *Connection) Close() {
	c.inFile.Close()
	c.outFile.Close()

	// Close all channels, ignore any panics due to already being closed
	c.channelsLock.Lock()
	defer c.channelsLock.Unlock()
	for _, v := range c.responseChannels {
		func() {
			/*
				defer func() {
					if r := recover(); r != nil {
					}
				}()
			*/
			close(v)
		}()
	}
}

func ReadNum(in *os.File) int {
	num := 0
	ch := []byte{'\000'}
	for {
		n, err := in.Read(ch)
		if n < 0 || err != nil {
			// fmt.Printf("%sErr in readnum(%d): %q\n", LogPrefix, n, err)
			return -1
		}
		if ch[0] == 0 {
			return num
		}
		num = (num * 10) + int(ch[0]-byte('0'))
	}
}

func ReadStr(in *os.File) string {
	str := ""
	ch := []byte{'\000'}
	for {
		n, err := in.Read(ch)
		if n < 0 || err != nil {
			// fmt.Printf("%sErr in readstr(%d): %q\n", LogPrefix, n, err)
			return "error"
		}
		if ch[0] == ' ' {
			return str
		}
		str = str + string(ch)
	}
}

func AddNum(buf []byte, num int) []byte {
	tmpBuf := []byte{'\000'}
	for num > 0 {
		tmpBuf = append([]byte{byte('0') + byte(num%10)}, tmpBuf...)
		num = num / 10
	}
	return append(buf, tmpBuf...)
}

func AddStr(buf []byte, str string) []byte {
	buf = append(buf, []byte(str)...)
	return append(buf, ' ')
}

func (c *Connection) ReadChunk() (*Chunk, error) {
	c.readLock.Lock()
	defer c.readLock.Unlock()

	chunk := Chunk{}
	size := 0

	chunk.threadID = ReadNum(c.inFile)
	chunk.direction = ReadNum(c.inFile)
	chunk.cmd = ReadStr(c.inFile)
	size = ReadNum(c.inFile)

	if size < 0 {
		return nil, fmt.Errorf("%sEOF1: %d", LogPrefix, size)
	}

	chunk.buffer = make([]byte, size)
	n, err := c.inFile.Read(chunk.buffer)
	if n != size || err != nil {
		return nil, fmt.Errorf("%sEOF2(%d,%d): %q", LogPrefix, n, size, err)
	}

	return &chunk, nil
}

func (c *Connection) WriteChunk(chunk *Chunk) error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	if chunk.threadID == 0 {
		chunk.threadID = UniqueID()
	}

	buf := []byte{}

	buf = AddNum(buf, chunk.threadID)
	buf = AddNum(buf, chunk.direction)
	buf = AddStr(buf, chunk.cmd)
	buf = AddNum(buf, len(chunk.buffer))

	n, err := c.outFile.Write(buf)
	if n != len(buf) || err != nil {
		return fmt.Errorf("%sEOF3(%d,%d): %q", LogPrefix, n, len(chunk.buffer), err)
	}

	n, err = c.outFile.Write(chunk.buffer)

	if n != len(chunk.buffer) || err != nil {
		return fmt.Errorf("%sEOF3(%d,%d): %q", LogPrefix, n, len(chunk.buffer), err)
	}

	return nil
}

func (c *Connection) IncomingProcessor() {
	for {
		chunk, err := c.ReadChunk()
		if err != nil {
			c.Close()
			// fmt.Printf("Error reading chunk: %q\n", err)
			break
		}

		if chunk.direction == Request || chunk.direction == Oneway {
			go c.ProcessRequest(chunk)
			continue
		}

		c.channelsLock.Lock()
		responseChannel := c.responseChannels[chunk.threadID]
		c.channelsLock.Unlock()

		if responseChannel == nil {
			// fmt.Printf("Can't find response channel")
			continue
		}

		responseChannel <- chunk
	}
}

func (c *Connection) ProcessRequest(inChunk *Chunk) {
	outChunk := c.reqFn(c, inChunk)

	if inChunk.direction == Request {
		if outChunk == nil {
			outChunk = &Chunk{}
		}
		if outChunk.direction == 0 {
			outChunk.direction = Response
		}
		if outChunk.threadID == 0 {
			outChunk.threadID = inChunk.threadID
		}
		if outChunk.cmd == "" {
			outChunk.cmd = inChunk.cmd
		}
		c.WriteChunk(outChunk)
	}
}

func (c *Connection) Call(cmd string, args string) (string, error) {
	var buf []byte
	if args != "" {
		buf = []byte(args)
	}

	buf, err := c.CallBytes(cmd, buf)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func (c *Connection) CallBytes(cmd string, buf []byte) ([]byte, error) {
	inChunk := Chunk{
		cmd:       cmd,
		direction: Request,
		buffer:    buf,
	}

	outChunk, err := c.CallChunk(&inChunk)
	if err != nil {
		return nil, err
	}

	return outChunk.buffer, nil
}

func (c *Connection) CallChunk(inChunk *Chunk) (*Chunk, error) {
	if inChunk.threadID == 0 {
		inChunk.threadID = UniqueID()
	}

	responseChannel := make(chan *Chunk)
	c.channelsLock.Lock()
	c.responseChannels[inChunk.threadID] = responseChannel
	c.channelsLock.Unlock()

	if err := c.WriteChunk(inChunk); err != nil {
		return nil, err
	}

	outChunk := <-responseChannel
	delete(c.responseChannels, inChunk.threadID)

	if outChunk == nil {
		return nil, fmt.Errorf("Missing response chunk")
	}

	if outChunk.cmd == CmdError {
		return nil, fmt.Errorf("%s", string(outChunk.buffer))
	}

	return outChunk, nil
}

func (c *Connection) Notify(cmd string, args string) error {
	var buf []byte
	if args != "" {
		buf = []byte(args)
	}

	return c.NotifyBytes(cmd, buf)
}

func (c *Connection) NotifyBytes(cmd string, buf []byte) error {
	inChunk := Chunk{
		cmd:       cmd,
		direction: Oneway,
		buffer:    buf,
	}

	return c.NotifyChunk(&inChunk)
}

func (c *Connection) NotifyChunk(inChunk *Chunk) error {
	return c.WriteChunk(inChunk)
}
