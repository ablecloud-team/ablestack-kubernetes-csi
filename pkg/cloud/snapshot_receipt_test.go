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
	"fmt"
	"github.com/ablecloud-team/ablestack-mold-go/v2/cloudstack"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSnapshotLookupsPreserveSizeAndCreationTime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"listsnapshotsresponse":{"count":1,"snapshot":[{"id":"snap-1","name":"snapshot-pvc","virtualsize":4294967296,"created":"2026-10-06T18:10:42+0000","volumeid":"vol-1"}]}}`)
	}))
	defer server.Close()
	c := New(&Config{APIURL: server.URL, APIKey: "fixture", SecretKey: "fixture", VerifySSL: true})
	for _, lookup := range []func(context.Context, string) (*Snapshot, error){c.GetSnapshotByID, c.GetSnapshotByName} {
		s, err := lookup(context.Background(), "snap-1")
		if err != nil || s.Size != 4294967296 || s.CreatedAt == "" {
			t.Fatalf("snapshot metadata lost: %+v %v", s, err)
		}
	}
}
func TestVolumeReceiptIncludesStateAndSnapshotTag(t *testing.T) {
	v := mapVolume(&cloudstack.Volume{Id: "vol-1", State: "Destroy", Tags: []cloudstack.Tags{{Key: "mold.csi.snapshot-id", Value: "snap-1"}}})
	if v.State != "Destroy" || v.SnapshotID != "snap-1" {
		t.Fatalf("invalid mapped receipt: %+v", v)
	}
}
