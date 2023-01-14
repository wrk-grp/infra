package infra

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/theapemachine/wrkspc/brazil"
	"github.com/theapemachine/wrkspc/tweaker"
	"github.com/wrk-grp/errnie"
)

type Manifest struct {
	scope  string
	values map[string]string
	f      string
}

type Values struct {
	Name      string
	Namespace string
	Data      string
	Port      string
}

func NewManifest(scope string, values map[string]string, f string) *Manifest {
	return &Manifest{scope, values, f}
}

func (manifest *Manifest) Compile() []byte {
	if manifest.scope != "services" {
		data, err := embedded.ReadFile(fmt.Sprintf(
			"cfg/%s/%s", manifest.scope, manifest.values["values"],
		))
		errnie.Handles(err)
		return data
	}

	errnie.Debugs(fmt.Sprintf(
		"compiling cfg/%s/%s.yml for %s",
		manifest.scope, manifest.f, manifest.values["name"],
	))

	if data, err := embedded.ReadFile(
		fmt.Sprintf("cfg/%s/%s.yml", manifest.scope, manifest.f),
	); errnie.Handles(err) == nil {
		tmpl, err := template.New(manifest.f).Parse(
			string(data),
		)

		mrg := ""
		if manifest.f == "configmap" {
			errnie.Handles(err)
			cfg := brazil.NewFile(
				"../"+manifest.values["name"]+"/cmd/cfg",
				"."+manifest.values["name"]+".yml",
				nil,
			)
			errnie.Handles(err)
			data, err := io.ReadAll(cfg.Data)
			errnie.Handles(err)

			str := string(data)

			for _, line := range strings.Split(str, "\n") {
				mrg += "  " + line + "\n"
			}
		}

		buf := bytes.NewBuffer([]byte{})
		tmpl.Execute(buf, Values{
			Name:      manifest.values["name"],
			Namespace: tweaker.Stage(),
			Data:      mrg,
			Port:      tweaker.GetString("server.port"),
		})

		return buf.Bytes()
	}

	return []byte{}
}

