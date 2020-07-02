package otp

import (
	"encoding/csv"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"
)

// randomSecret generate a random secret of given length
func randomSecret(length int) string {
	rand.Seed(time.Now().UnixNano()) // initialize global pseudo random generator
	letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")

	bytes := make([]rune, length)

	for i := range bytes {
		bytes[i] = letterRunes[rand.Intn(len(letterRunes))]
	}

	return string(bytes)
}

// randomTimeStep return a random value from an array of possible values
func randomTimeStep() uint64 {
	timesteps := make([]uint64, 0)
	timesteps = append(timesteps,
		30,
		60,
		1800)
	rand.Seed(time.Now().UnixNano()) // initialize global pseudo random generator
	return timesteps[rand.Intn(len(timesteps))]
}

// randomTimeStamp return a random timestamp as unix time (seconds since EPOCH)
func randomTimestamp() int64 {
	randomTime := rand.Int63n(time.Now().Unix()-94608000) + 94608000

	randomNow := time.Unix(randomTime, 0)

	return int64(randomNow.Unix())
}

func TestTOTPMatch(t *testing.T) {
	gen := TOTPGenerator{
		TimeStep:  randomTimeStep(),
		StartTime: randomTimestamp(),
		Secret:    randomSecret(64),
		Digit:     9,
	}

	if gen.At(currentTimestamp()) != gen.Now() {
		t.Fatalf("TOTPs do not match")
	}
}

func TestTOTPLength(t *testing.T) {

	// Generate 3 TOTPs for each possible length between 1 and 9
	for i := 1; i < 10; i++ {
		gen := TOTPGenerator{
			TimeStep:  randomTimeStep(),
			StartTime: randomTimestamp(),
			Secret:    randomSecret(64),
			Digit:     i,
		}

		otp := gen.At(currentTimestamp())

		if len(otp.OTP) != gen.Digit {
			t.Fatalf("Failed while expecting length to be %d but was %d", gen.Digit, len(otp.OTP))
		}
	}
}

func TestTOTPRotation(t *testing.T) {
	// Ensure TOTP generated just before and just after are different from current
	timeStep := randomTimeStep()
	startTime := randomTimestamp()
	secret := randomSecret(64)
	now := currentTimestamp()
	before := now - int64(timeStep)
	after := now + int64(timeStep)
	gen := TOTPGenerator{
		TimeStep:  timeStep,
		StartTime: startTime,
		Secret:    secret,
		Digit:     9,
	}

	otp := gen.At(now)
	otpNext := gen.At(after)
	otpPrevious := gen.At(before)

	if otp == otpNext || otp == otpPrevious {
		t.Fatalf("Test failed for TOTP rotation")
	}
}

func TestKnownTOTPs(t *testing.T) {

	f, err := os.Open("totps.csv")

	if err != nil {
		fmt.Println(err)
	}
	defer f.Close()

	csvReader := csv.NewReader(f)

	// Read first line
	_, err = csvReader.Read()

	for {
		// Read each record from csv
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}

		secret := record[0]
		timeStep, _ := strconv.ParseUint(record[1], 10, 64)
		startTime, _ := strconv.ParseInt(record[2], 10, 64)
		digits, _ := strconv.Atoi(record[3])
		generateAt, _ := strconv.ParseInt(record[4], 10, 64)
		expectedResult := record[5]

		var gen = TOTPGenerator{
			TimeStep:  timeStep,
			StartTime: startTime,
			Secret:    secret,
			Digit:     digits,
		}

		var otp = gen.At(generateAt)

		if otp.OTP != expectedResult {
			t.Fatalf("Test failed for known TOTPS. For secret %s, timestep %d, starttime %d, digits %d generated at %d it should be %s but was %s", secret, timeStep, startTime, digits, generateAt, expectedResult, otp.OTP)
		}
	}
}
