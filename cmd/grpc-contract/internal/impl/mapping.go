package impl

import (
	"fmt"
	"os"
	"bytes"
	parser "github.com/zpatrick/go-parser"
	"text/template"
)

var typeMaps = map[string]map[string]string{
	"[]byte": {
		"*big.Int": `new(big.Int).SetBytes({{ .Input }})`,
		"[32]byte": `grpc.BytesToBytes32({{ .Input }})`,
	},
	"string": {
		"common.Address": `common.HexToAddress({{ .Input }})`,
		"[]byte":         `[]byte({{ .Input }})`,
	},
	"*big.Int": {
		"[]byte": `{{ .Input }}.Bytes()`,
	},
	"[][]byte": {
		"[]*big.Int": `grpc.BytesToBigIntArray({{ .Input }})`,
		"[][32]byte": `grpc.BytesArrayToBytes32Array({{ .Input }})`,
	},
	"[]*big.Int": {
		"[][]byte": `grpc.BigIntArrayToBytes({{ .Input }})`,
	},
	"[32]byte": {
		"[]byte": `{{ .Input }}[:]`,
		"string": `string({{ .Input }}[:])`,
	},
	"common.Address": {
		"string": `{{ .Input }}.Hex()`,
	},
}

type TypeMap struct {
	Input    string
	Template string
}

func NewTypeMap(in, inType, outType string) *TypeMap {
	if inType == outType {
		return &TypeMap{
			Input:    in,
			Template: "{{ .Input }}",
		}
	}
	temp, ok := typeMaps[inType][outType]
	if !ok {
		return nil
	}
	return &TypeMap{
		Input:    in,
		Template: temp,
	}
}

func (t *TypeMap) String() string {
	implTemplate, err := template.New("type_map").Parse(t.Template)
	if err != nil {
		fmt.Printf("Failed to parse template: %v\n", err)
		os.Exit(-1)
	}
	result := new(bytes.Buffer)
	implTemplate.Execute(result, t)
	return result.String()
}

func toRequestParam(f *parser.GoField, t *parser.GoType) string {
	typeMapping := NewTypeMap("r.Get"+f.Name+"()", f.Type, t.Type)
	if typeMapping == nil {
		panic("cannot find corresponding request type, from: " + f.Type + ", to: " + t.Type)
	}
	return typeMapping.String()
}

func toResponseParam(input string, t *parser.GoType, f *parser.GoField) string {
	typeMapping := NewTypeMap(input, t.Type, f.Type)
	if typeMapping == nil {
		panic("cannot find corresponding response type, from: " + t.Type + ", to: " + f.Type)
	}
	return fmt.Sprintf("%v : %v", f.Name, typeMapping.String())
}
