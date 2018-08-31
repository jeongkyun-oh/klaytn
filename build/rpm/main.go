package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"text/template"
)

type RpmSpec struct {
	BuildNumber int
	Version     string
}

func (r RpmSpec) String() string {
	tmpl, err := template.New("rpmspec").Parse(rpmSpecTemplate)
	if err != nil {
		fmt.Printf("Failed to parse template, %v", err)
		return ""
	}

	result := new(bytes.Buffer)
	err = tmpl.Execute(result, r)
	if err != nil {
		fmt.Printf("Failed to render template, %v", err)
		return ""
	}
	return result.String()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Error : genrpmspec <buildNumber>")
		os.Exit(1)
	}
	version := ""
	if len(os.Args) == 3 {
		version = os.Args[2]
	}
	rpmSpec := new(RpmSpec)
	buildNumber, err := strconv.Atoi(os.Args[1])

	if err != nil {
		fmt.Printf("BuildNumber must be int, %v", err)
		os.Exit(1)
	}
	rpmSpec.BuildNumber = buildNumber
	if version != "" {
		rpmSpec.Version = version
	} else {
		rpmSpec.Version = "devel"
	}
	fmt.Println(rpmSpec)
}

var rpmSpecTemplate = `Name:               go-klaytn
Version:            {{ .Version }}
Release:            {{ .BuildNumber }}%{?dist}
Summary:            the go-klaytn package

Group:              Application/blockchain
License:            GNU
URL:                http://www.klaytn.io
Source0:            %{name}-%{version}.tar.gz
BuildRoot:          %(mktemp -ud %{_tmppath}/%{name}-%{version}-%{release}-XXXXXX)

%description
 The Klaytn blockchain platform

%prep
%setup -q

%build
make klay

%install
mkdir -p $RPM_BUILD_ROOT/usr/local/bin
mkdir -p $RPM_BUILD_ROOT/etc/klay/conf
mkdir -p $RPM_BUILD_ROOT/etc/init.d/
mkdir -p $RPM_BUILD_ROOT/var/log/klay

cp build/bin/klay $RPM_BUILD_ROOT/usr/local/bin/klay
cp build/rpm/etc/init.d/klay $RPM_BUILD_ROOT/etc/init.d/klay
cp build/rpm/etc/klay/conf/klay.conf $RPM_BUILD_ROOT/etc/klay/conf/klay.conf

%files
%attr(754, -, -) /usr/local/bin/klay
%attr(644, -, -) /etc/klay/conf/klay.conf
%attr(754, -, -) /etc/init.d/klay
`
