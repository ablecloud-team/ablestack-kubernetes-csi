//
// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
//

package driver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublishedCsiProfileMatchesFilesAndHostDeviceIdentity(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "profiles", "mold-gfs2-amd64")
	raw, err := os.ReadFile(filepath.Join(root, "profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	var profile struct {
		DriverImage string            `json:"driverImage"`
		Files       map[string]string `json:"files"`
		Images      map[string]string `json:"images"`
	}
	if err = json.Unmarshal(raw, &profile); err != nil {
		t.Fatal(err)
	}
	if len(profile.Images) != 8 {
		t.Fatalf("expected 8 locked CSI images, got %d", len(profile.Images))
	}
	for name, digest := range profile.Files {
		b, e := os.ReadFile(filepath.Join(root, name))
		if e != nil {
			t.Fatal(e)
		}
		h := sha256.Sum256(b)
		if hex.EncodeToString(h[:]) != digest {
			t.Fatalf("profile checksum mismatch: %s", name)
		}
	}
	raw, err = os.ReadFile(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	foundNode := false
	for {
		var d map[string]interface{}
		err = dec.Decode(&d)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		kind, _ := d["kind"].(string)
		if kind == "Secret" {
			t.Fatal("published profile must not contain a credential Secret")
		}
		if kind == "ServiceAccount" {
			if _, ok := d["spec"]; ok {
				t.Fatal("invalid ServiceAccount spec")
			}
		}
		if kind != "DaemonSet" && kind != "Deployment" {
			continue
		}
		spec := d["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})
		ports := map[string]bool{}
		for _, value := range spec["containers"].([]interface{}) {
			c := value.(map[string]interface{})
			image := c["image"].(string)
			if !strings.Contains(image, "@sha256:") {
				t.Fatalf("mutable image: %s", image)
			}
			if values, ok := c["ports"].([]interface{}); ok {
				for _, value := range values {
					name := value.(map[string]interface{})["name"].(string)
					if len(name) > 15 || ports[name] {
						t.Fatalf("invalid or duplicate port name: %s", name)
					}
					ports[name] = true
				}
			}
			if c["name"] == "cloudstack-csi-node" {
				foundNode = true
				if image != profile.DriverImage {
					t.Fatal("driver image mismatch")
				}
				readonlyUdev := false
				for _, value := range c["volumeMounts"].([]interface{}) {
					m := value.(map[string]interface{})
					if m["name"] == "udev-dir" && m["mountPath"] == "/run/udev" && m["readOnly"] == true {
						readonlyUdev = true
					}
				}
				if !readonlyUdev {
					t.Fatal("serial validation requires the host udev database read-only")
				}
				hostUdev := false
				for _, value := range spec["volumes"].([]interface{}) {
					v := value.(map[string]interface{})
					if v["name"] == "udev-dir" && v["hostPath"].(map[string]interface{})["path"] == "/run/udev" {
						hostUdev = true
					}
				}
				if !hostUdev {
					t.Fatal("host udev volume missing")
				}
			}
		}
	}
	if !foundNode {
		t.Fatal("node driver missing")
	}
}
