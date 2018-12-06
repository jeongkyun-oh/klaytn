package grpc

import (
	"fmt"
	"github.com/ground-x/go-gxplatform/accounts/abi"
	"github.com/ground-x/go-gxplatform/cmd/utils"
	"html/template"
	"io"
	"sort"
)

// Generate a renderable object and required message types from an GXP contract ABI
func GenerateServiceProtoFile(srvName, pkgName string, contractABI abi.ABI, version string) (protoFile ProtoFile, msgs []Message) {
	protoFile = ProtoFile{
		GeneratorVersion: version,
		Package:          pkgName,
		Name:             utils.ToCamelCase(srvName),
		Sources:          []string{fmt.Sprintf("%s.abi", srvName)},
	}

	methods, requiredMsgs := ParseMethods(contractABI.Methods)
	protoFile.Methods = append(protoFile.Methods, methods...)

	msgs = append(msgs, requiredMsgs...)

	events, requiredMsgs := ParseEvents(contractABI.Events)
	protoFile.Events = append(protoFile.Events, events...)

	msgs = append(msgs, requiredMsgs...)

	sort.Sort(protoFile.Methods)
	sort.Sort(protoFile.Events)
	sort.Sort(protoFile.Sources)

	return protoFile, msgs
}

type ProtoFile struct {
	GeneratorVersion string
	Package          string
	Name             string
	Methods          Methods
	Events           Methods
	Sources          Sources
}

func (p ProtoFile) Render(writer io.WriteCloser) error {
	template, err := template.New("proto").Parse(ServiceTemplate)
	if err != nil {
		fmt.Printf("Failed to parse template: %v\n", err)
		return err
	}

	return template.Execute(writer, p)
}

var ServiceTemplate string = `// Automatically generated by sol2proto {{ .GeneratorVersion }}. DO NOT EDIT!
// sources: {{ range .Sources }}
//     {{ . }}
{{- end }}
syntax = "proto3";

package {{ .Package }};

import "messages.proto";

service {{ .Name }} {
{{- range .Methods }}
    {{ . }}
{{- end }}

    // Not supported yet
{{- range .Events }}
    // {{ . }}
{{- end }}
}
`
