package otp

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"time"
)

const (
	fourBitMask      = 0xf
	eightBitMask     = 0xff
	thirtyOneBitMask = 0x7fffffff
)

// HOTPGenerator for HMAC-based One-Time Password generation
type HOTPGenerator struct {
	Counter uint64 // C in RFC4226
	Secret  string // K in RFC4226
	Digit   int
}

// TOTPGenerator for Time-based One-Time Password generation
type TOTPGenerator struct {
	TimeStep  uint64 // X in RFC6238
	StartTime int64  // T0 in RFC6238, default 0 is OK
	Secret    string // shared secret for HMAC
	Digit     int
}

// TOTP is a representation of a time-based one time password
type TOTP struct {
	TimeStamp int64
	OTP       string
}

// generate generate HOTP from generator
func (g *HOTPGenerator) generate() int64 {
	hs := hmacSHA1([]byte(g.Secret), counterToBytes(g.Counter))
	snum := truncate(hs)
	d := int64(snum) % int64(math.Pow10(g.Digit))
	g.Counter++
	return d
}

// counterToBytes transform a counter into an array of bytes
func counterToBytes(c uint64) []byte {
	t := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		t[i] = byte(c & eightBitMask)
		c = c >> 8
	}
	return t
}

// hmacSHA1 return the hash of a given message with a given key
func hmacSHA1(k []byte, c []byte) (hs []byte) {
	mac := hmac.New(sha1.New, k)
	mac.Write(c)
	hs = mac.Sum(nil)
	return hs
}

func truncate(hs []byte) int {
	offsetBits := hs[len(hs)-1] & fourBitMask
	offset := int(offsetBits)
	p := hs[offset : offset+4]
	return int(binary.BigEndian.Uint32(p)) & thirtyOneBitMask
}

// currentTimestamp returns the current time as number of seconds since epoch
func currentTimestamp() int64 {
	return int64(time.Now().Unix())
}

// generate generate TOTP from the generator
func (g TOTPGenerator) generate() int64 {
	if g.TimeStep == 0 {
		g.TimeStep = 30 // default to 30 seconds
	}

	// the counter is the number of elapsed time steps between now and start time
	now := time.Now().UTC().Unix()
	t := (now - g.StartTime) / int64(g.TimeStep)

	h := HOTPGenerator{
		Secret:  g.Secret,
		Digit:   g.Digit,
		Counter: uint64(t),
	}
	return h.generate()
}

// genereateAt generates TOTP from generator for the specific timestamp
func (g TOTPGenerator) generateAt(timestamp int64) int64 {
	if g.TimeStep == 0 {
		g.TimeStep = 30 // default to 30 seconds
	}

	// the counter is the number of elapsed time steps between now and start time
	t := (timestamp - g.StartTime) / int64(g.TimeStep)

	h := HOTPGenerator{
		Secret:  g.Secret,
		Digit:   g.Digit,
		Counter: uint64(t),
	}
	return h.generate()
}

// At generates TOTP for specific timestamp
func (g TOTPGenerator) At(timestamp int64) TOTP {
	// format the generated OTP as a string which length corresponds to the desired number of digits
	var otp = fmt.Sprintf("%0"+strconv.Itoa(g.Digit)+"d", g.generateAt(timestamp))

	return TOTP{
		TimeStamp: timestamp,
		OTP:       otp,
	}
}

// Now generates TOTP for current timestamp
func (g TOTPGenerator) Now() TOTP {
	var ts = currentTimestamp()
	// format the generated OTP as a string which length corresponds to the desired number of digits
	var otp = fmt.Sprintf("%0"+strconv.Itoa(g.Digit)+"d", g.generateAt(ts))

	return TOTP{
		TimeStamp: ts,
		OTP:       otp,
	}
}
