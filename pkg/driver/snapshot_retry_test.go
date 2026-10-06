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
	"context"
	"github.com/ablecloud-team/ablestack-kubernetes-csi/pkg/cloud"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

type existingVolumeConnector struct {
	cloud.Interface
	volume *cloud.Volume
}

func (c existingVolumeConnector) GetVolumeByName(context.Context, string) (*cloud.Volume, error) {
	return c.volume, nil
}
func TestRestoreRetryChecksStateAndContentSource(t *testing.T) {
	for _, tc := range []struct {
		name, state, source string
		want                codes.Code
	}{
		{"complete", "Ready", "snapshot-1", codes.OK},
		{"failed", "Destroy", "snapshot-1", codes.FailedPrecondition},
		{"partial", "Creating", "snapshot-1", codes.FailedPrecondition},
		{"unproven", "Ready", "", codes.AlreadyExists},
		{"different", "Ready", "snapshot-2", codes.AlreadyExists},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cs := NewControllerServer(existingVolumeConnector{volume: &cloud.Volume{ID: "volume-1", Name: "pvc-1", Size: 4294967296, State: tc.state, SnapshotID: tc.source, DiskOfferingID: "gfs2", ZoneID: "zone-1"}})
			src := &csi.VolumeContentSource{Type: &csi.VolumeContentSource_Snapshot{Snapshot: &csi.VolumeContentSource_SnapshotSource{SnapshotId: "snapshot-1"}}}
			req := &csi.CreateVolumeRequest{Name: "pvc-1", Parameters: map[string]string{DiskOfferingKey: "gfs2"}, VolumeContentSource: src, VolumeCapabilities: []*csi.VolumeCapability{{AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}}, AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER}}}}
			response, err := cs.CreateVolume(context.Background(), req)
			if status.Code(err) != tc.want {
				t.Fatalf("want %v got %v", tc.want, err)
			}
			if tc.want == codes.OK && (response.Volume.ContentSource.GetSnapshot().GetSnapshotId() != "snapshot-1" || response.Volume.CapacityBytes != 4294967296) {
				t.Fatal("successful retry lost content source or size")
			}
		})
	}
}
