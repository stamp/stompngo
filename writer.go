//
// Copyright © 2011-2013 Guy M. Allard
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package stompngo

import (
	"bufio"
	"fmt"
	"strconv"
	"time"
)

/*
	Logical network writer.  Read wiredata structures from the communication
	channel, and put them on the wire.
*/
func (c *Connection) writer() {
	q := false
	for {

		select {
		case d := <-c.output:
			c.wireWrite(d)
		case q = <-c.wsd:
			break
		}

		if q {
			break
		}

	}
	c.log("writer shutdown", time.Now())
}

/*
	Connection logical write.
*/
func (c *Connection) wireWrite(d wiredata) {
	f := d.frame
	switch f.Command {
	case "\n": // HeartBeat frame
		if _, e := fmt.Fprintf(c.wtr, "%s", f.Command); e != nil {
			d.errchan <- e
			return
		}
	default: // Other frames
		if e := f.writeFrame(c.wtr, c.Protocol()); e != nil {
			d.errchan <- e
			return
		}
		if e := c.wtr.WriteByte('\x00'); e != nil {
			d.errchan <- e
			return
		}
	}
	if e := c.wtr.Flush(); e != nil {
		d.errchan <- e
		return
	}
	//
	if c.hbd != nil {
		c.hbd.ls = time.Now().UnixNano() // Latest good send
	}
	c.mets.tfw += 1             // Frame written count
	m := Message(f)             // Convert
	c.mets.tbw += m.Size(false) // Bytes written count
	//
	d.errchan <- nil
	return
}

/*
	Frame physical write.
*/
func (f *Frame) writeFrame(w *bufio.Writer, l string) error {
	// Write the frame Command
	if _, e := fmt.Fprintf(w, "%s\n", f.Command); e != nil {
		return e
	}
	// Content length - Always add it if client does not suppress it and
	// does not supply it.
	if _, ok := f.Headers.Contains("suppress-content-length"); !ok {
		if _, clok := f.Headers.Contains("content-length"); !clok {
			l := strconv.Itoa(len(f.Body))
			f.Headers = f.Headers.Add("content-length", l)
		}
	}
	// Write the frame Headers
	for i := 0; i < len(f.Headers); i += 2 {
		if l > SPL_10 && f.Command != CONNECT {
			f.Headers[i] = encode(f.Headers[i])
			f.Headers[i+1] = encode(f.Headers[i+1])
		}
		_, e := fmt.Fprintf(w, "%s:%s\n", f.Headers[i], f.Headers[i+1])
		if e != nil {
			return e
		}
	}
	// Write the last Header LF and the frame Body
	if _, e := fmt.Fprintf(w, "\n%s", f.Body); e != nil {
		return e
	}
	return nil
}
