// Copyright The Kubernetes Authors.
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

package unit

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func TestImageBuildAndPublishingReferences(t *testing.T) {
	for _, repository := range []string{"Registry.Example/Team/Images", "Registry.Example:5000/Team/Images"} {
		for _, target := range []string{"container", "push", "push-multi-arch"} {
			t.Run(repository+"/"+target, func(t *testing.T) {
				dir := t.TempDir()
				cli := filepath.Join(dir, "container-cli")
				script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$IMAGE_COMMAND_LOG\"\n"
				if err := os.WriteFile(cli, []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
				log := filepath.Join(dir, "commands")
				command := exec.Command("make", "-s", target, "CONTAINER_CLI="+cli, "REGISTRY="+repository,
					"ARCH=amd64", "GIT_TAG=ReleaseRC1", "CHECKSUM=BuildCheckSUM", "ALL_ARCHITECTURES=amd64 arm64")
				command.Dir = filepath.Join("..", "..")
				command.Env = append(os.Environ(), "IMAGE_COMMAND_LOG="+log)
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("make %s: %v\n%s", target, err, output)
				}
				output, err := os.ReadFile(log)
				if err != nil {
					t.Fatal(err)
				}
				actual := string(output)
				want := strings.ToLower(repository) + "/metrics-server"
				if strings.Contains(actual, repository) || !strings.Contains(actual, want) {
					t.Fatalf("repository not normalized: %s", actual)
				}
				if target == "container" {
					if !strings.Contains(actual, want+"-amd64:BuildCheckSUM") {
						t.Fatalf("checksum tag changed: %s", actual)
					}
				} else if !strings.Contains(actual, ":ReleaseRC1") {
					t.Fatalf("release tag changed: %s", actual)
				}
			})
		}
	}
}

func TestHelmImageRepositoryCase(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "charts", "metrics-server", "templates", "_helpers.tpl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"metrics-server.image", "metrics-server.addonResizer.image"} {
		t.Run(name, func(t *testing.T) {
			_, body, ok := strings.Cut(string(content), `{{- define "`+name+`" -}}`)
			if !ok {
				t.Fatalf("missing image helper %s", name)
			}
			body, _, ok = strings.Cut(body, "{{- end }}")
			if !ok {
				t.Fatal("unterminated image helper")
			}
			funcs := template.FuncMap{"lower": strings.ToLower, "default": func(fallback, value string) string {
				if value == "" {
					return fallback
				}
				return value
			}}
			tmpl, err := template.New("image").Funcs(funcs).Parse(body)
			if err != nil {
				t.Fatal(err)
			}
			for _, tag := range []string{"ReleaseRC1", ""} {
				image := map[string]any{"repository": "Registry.Example:5000/Team/Metrics", "tag": tag}
				ctx := map[string]any{"Chart": map[string]any{"AppVersion": "0.9.0-RC1"}, "Values": map[string]any{
					"image": image, "addonResizer": map[string]any{"image": image},
				}}
				var output bytes.Buffer
				if err := tmpl.Execute(&output, ctx); err != nil {
					t.Fatal(err)
				}
				expectedTag := tag
				if name == "metrics-server.image" && tag == "" {
					expectedTag = "v0.9.0-RC1"
				}
				expected := "registry.example:5000/team/metrics:" + expectedTag
				if strings.TrimSpace(output.String()) != expected {
					t.Fatalf("got %q; want %q", output.String(), expected)
				}
			}
		})
	}
}
