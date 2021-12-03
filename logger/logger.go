package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
)

type LogLevel string
const (
    LvlDebug = LogLevel("DEBUG")
    LvlError = LogLevel("ERROR")
    LvlInfo  = LogLevel("INFO")
)

// PrintMarshal pretty prints a struct for use in debugging
func PrintMarshal(level LogLevel, msg string, v interface{}) {
	log.Printf("%s %s\n", level, msg)
	pp, _ := json.MarshalIndent(v, "", "    ")
	fmt.Println(string(pp))
}

// PrintJSONBytes pretty prints a JSON byte array
func PrintJSONBytes(level LogLevel, msg string, v []byte) {
	var out bytes.Buffer
	err := json.Indent(&out, v, "", "    ")
	if err != nil {
		log.Printf("error indenting JSON []byte: %v", err)
		return
	}
	log.Printf("%s %s %s\n", level, msg, out.String())
}

//Print allows for generic debug printing
func Printf(level LogLevel, msg string, v ...interface{}) {
	log.Printf("%s %s%v", level, msg, v)
}
