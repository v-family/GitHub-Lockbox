/*
 * Copyright (c) 2016, Psiphon Inc.
 * All rights reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package common

import (
	"bytes"
	"compress/zlib"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"runtime"
	"strings"
	"time"

	"github.com/Psiphon-Labs/psiphon-tunnel-core/psiphon/common/wildcard"
)

const RFC3339Milli = "2006-01-02T15:04:05.000Z07:00"

// Contains is a helper function that returns true
// if the target string is in the list.
func Contains(list []string, target string) bool {
	for _, listItem := range list {
		if listItem == target {
			return true
		}
	}
	return false
}

// ContainsWildcard returns true if target matches
// any of the patterns. Patterns may contain the
// '*' wildcard.
func ContainsWildcard(patterns []string, target string) bool {
	for _, pattern := range patterns {
		if wildcard.Match(pattern, target) {
			return true
		}
	}
	return false
}

// ContainsAny returns true if any string in targets
// is present in the list.
func ContainsAny(list, targets []string) bool {
	for _, target := range targets {
		if Contains(list, target) {
			return true
		}
	}
	return false
}

// ContainsInt returns true if the target int is
// in the list.
func ContainsInt(list []int, target int) bool {
	for _, listItem := range list {
		if listItem == target {
			return true
		}
	}
	return false
}

// GetStringSlice converts an interface{} which is
// of type []interace{}, and with the type of each
// element a string, to []string.
func GetStringSlice(value interface{}) ([]string, bool) {
	slice, ok := value.([]interface{})
	if !ok {
		return nil, false
	}
	strSlice := make([]string, len(slice))
	for index, element := range slice {
		str, ok := element.(string)
		if !ok {
			return nil, false
		}
		strSlice[index] = str
	}
	return strSlice, true
}

// MakeSecureRandomBytes is a helper function that wraps
// crypto/rand.Read.
func MakeSecureRandomBytes(length int) ([]byte, error) {
	randomBytes := make([]byte, length)
	n, err := rand.Read(randomBytes)
	if err != nil {
		return nil, ContextError(err)
	}
	if n != length {
		return nil, ContextError(errors.New("insufficient random bytes"))
	}
	return randomBytes, nil
}

// GetCurrentTimestamp returns the current time in UTC as
// an RFC 3339 formatted string.
func GetCurrentTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// TruncateTimestampToHour truncates an RFC 3339 formatted string
// to hour granularity. If the input is not a valid format, the
// result is "".
func TruncateTimestampToHour(timestamp string) string {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return ""
	}
	return t.Truncate(1 * time.Hour).Format(time.RFC3339)
}

// getFunctionName is a helper that extracts a simple function name from
// full name returned byruntime.Func.Name(). This is used to declutter
// log messages containing function names.
func getFunctionName(pc uintptr) string {
	funcName := runtime.FuncForPC(pc).Name()
	index := strings.LastIndex(funcName, "/")
	if index != -1 {
		funcName = funcName[index+1:]
	}
	return funcName
}

// GetParentContext returns the parent function name and source file
// line number.
func GetParentContext() string {
	pc, _, line, _ := runtime.Caller(2)
	return fmt.Sprintf("%s#%d", getFunctionName(pc), line)
}

// ContextError prefixes an error message with the current function
// name and source file line number.
func ContextError(err error) error {
	if err == nil {
		return nil
	}
	pc, _, line, _ := runtime.Caller(1)
	return fmt.Errorf("%s#%d: %s", getFunctionName(pc), line, err)
}

// ContextErrorMsg works like ContextError, but adds a message string to
// the error message.
func ContextErrorMsg(err error, message string) error {
	if err == nil {
		return nil
	}
	pc, _, line, _ := runtime.Caller(1)
	return fmt.Errorf("%s#%d: %s: %s", getFunctionName(pc), line, message, err)
}

// Compress returns zlib compressed data
func Compress(data []byte) []byte {
	var compressedData bytes.Buffer
	writer := zlib.NewWriter(&compressedData)
	writer.Write(data)
	writer.Close()
	return compressedData.Bytes()
}

// Decompress returns zlib decompressed data
func Decompress(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, ContextError(err)
	}
	uncompressedData, err := ioutil.ReadAll(reader)
	reader.Close()
	if err != nil {
		return nil, ContextError(err)
	}
	return uncompressedData, nil
}

// FormatByteCount returns a string representation of the specified
// byte count in conventional, human-readable format.
func FormatByteCount(bytes uint64) string {
	// Based on: https://bitbucket.org/psiphon/psiphon-circumvention-system/src/b2884b0d0a491e55420ed1888aea20d00fefdb45/Android/app/src/main/java/com/psiphon3/psiphonlibrary/Utils.java?at=default#Utils.java-646
	base := uint64(1024)
	if bytes < base {
		return fmt.Sprintf("%dB", bytes)
	}
	exp := int(math.Log(float64(bytes)) / math.Log(float64(base)))
	return fmt.Sprintf(
		"%.1f%c", float64(bytes)/math.Pow(float64(base), float64(exp)), "KMGTPEZ"[exp-1])
}

func CopyNBuffer(dst io.Writer, src io.Reader, n int64, buf []byte) (written int64, err error) {
	// Based on io.CopyN:
	// https://github.com/golang/go/blob/release-branch.go1.11/src/io/io.go#L339
	written, err = io.CopyBuffer(dst, io.LimitReader(src, n), buf)
	if written == n {
		return n, nil
	}
	if written < n && err == nil {
		err = io.EOF
	}
	return
}