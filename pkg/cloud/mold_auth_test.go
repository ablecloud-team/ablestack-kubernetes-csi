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

// Package cloud contains CloudStack related
// functions.
package cloud

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
)

func TestNodeAndVolumeLookupUseMoldSHA256(t *testing.T) {
	calls := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
			return
		}
		parameters := r.Form
		keys := []string{}
		for key := range parameters {
			if !strings.EqualFold(key, "signature") {
				keys = append(keys, key)
			}
		}
		sort.Slice(keys, func(i, j int) bool { return strings.ToLower(keys[i]) < strings.ToLower(keys[j]) })
		pairs := []string{}
		for _, key := range keys {
			pairs = append(pairs, key+"="+strings.ReplaceAll(url.QueryEscape(parameters.Get(key)), "+", "%20"))
		}
		mac := hmac.New(sha256.New, []byte("test-secret+special"))
		_, _ = mac.Write([]byte(strings.ToLower(strings.Join(pairs, "&"))))
		if parameters.Get("apiKey") != "test-api-key" || parameters.Get("signature") != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
			t.Error("unexpected Mold signature")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		command := strings.ToLower(parameters.Get("command"))
		calls[command]++
		w.Header().Set("Content-Type", "application/json")
		switch command {
		case "listvirtualmachines":
			fmt.Fprint(w, `{"listvirtualmachinesresponse":{"count":1,"virtualmachine":[{"id":"vm-1","zoneid":"zone-1"}]}}`)
		case "listvolumes":
			fmt.Fprint(w, `{"listvolumesresponse":{"count":1,"volume":[{"id":"vol-1","zoneid":"zone-1","size":1073741824}]}}`)
		default:
			t.Errorf("unexpected command %s", command)
			http.Error(w, "unexpected", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	c := New(&Config{APIURL: server.URL, APIKey: "test-api-key", SecretKey: "test-secret+special", VerifySSL: true})
	vm, err := c.GetVMByID(context.Background(), "vm-1")
	if err != nil || vm.ID != "vm-1" || vm.ZoneID != "zone-1" {
		t.Fatalf("node lookup failed: %v", err)
	}
	volume, err := c.GetVolumeByID(context.Background(), "vol-1")
	if err != nil || volume.ID != "vol-1" || volume.Size != 1073741824 {
		t.Fatalf("volume lookup failed: %v", err)
	}
	if calls["listvirtualmachines"] != 1 || calls["listvolumes"] != 1 {
		t.Fatalf("unexpected API requests: %v", calls)
	}
}
