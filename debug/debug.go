package debug

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
)

// PrintMarshal pretty prints a struct for use in debugging
func PrintMarshal(msg string, v interface{}) {
	log.Println(msg)
	pp, _ := json.MarshalIndent(v, "", "    ")
	fmt.Println(pp)
}

// PrintJSONBytes pretty prints a JSON byte array
func PrintJSONBytes(msg string, v []byte) {
	var out bytes.Buffer
	json.Indent(&out, v, "", "    ")
	log.Printf("%s: %s\n", msg, out.String())
}
