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
	"errors"
	"github.com/ablecloud-team/ablestack-kubernetes-csi/pkg/cloud"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

type snapshotConnector struct {
	cloud.Interface
	snapshot               *cloud.Snapshot
	lookupErr              error
	creates, volumeLookups int
}

func (c *snapshotConnector) GetSnapshotByName(context.Context, string) (*cloud.Snapshot, error) {
	if c.lookupErr != nil {
		return nil, c.lookupErr
	}
	if c.snapshot == nil {
		return nil, cloud.ErrNotFound
	}
	return c.snapshot, nil
}
func (c *snapshotConnector) GetVolumeByID(context.Context, string) (*cloud.Volume, error) {
	c.volumeLookups++
	return &cloud.Volume{ID: "vol-1", Size: 4294967296}, nil
}
func (c *snapshotConnector) CreateSnapshot(context.Context, string, string) (*cloud.Snapshot, error) {
	c.creates++
	c.snapshot = &cloud.Snapshot{ID: "snap-1", VolumeID: "vol-1", Name: "snapshot-pvc", Size: 4294967296, State: "BackedUp", CreatedAt: "2026-10-06T18:10:42+0000"}
	return c.snapshot, nil
}
func (c *snapshotConnector) ListSnapshots(context.Context, string, string) ([]*cloud.Snapshot, error) {
	return []*cloud.Snapshot{c.snapshot}, nil
}
func TestSnapshotCreateRetryAndListPreserveDurableReceipt(t *testing.T) {
	connector := &snapshotConnector{}
	controller := NewControllerServer(connector)
	request := &csi.CreateSnapshotRequest{Name: "snapshot-pvc", SourceVolumeId: "vol-1"}
	for i := 0; i < 2; i++ {
		result, err := controller.CreateSnapshot(context.Background(), request)
		if err != nil || result.Snapshot.SizeBytes != 4294967296 || !result.Snapshot.ReadyToUse || result.Snapshot.SnapshotId != "snap-1" {
			t.Fatalf("invalid receipt: %+v %v", result, err)
		}
	}
	if connector.creates != 1 || connector.volumeLookups != 1 {
		t.Fatalf("retry accessed source or duplicated snapshot: %+v", connector)
	}
	result, err := controller.ListSnapshots(context.Background(), &csi.ListSnapshotsRequest{})
	if err != nil || len(result.Entries) != 1 || result.Entries[0].Snapshot.SizeBytes != 4294967296 || !result.Entries[0].Snapshot.ReadyToUse {
		t.Fatalf("invalid list receipt: %+v %v", result, err)
	}
}
func TestSnapshotRetryFailsClosedForWrongSourceOrUnusableReceipt(t *testing.T) {
	for _, tc := range []struct {
		name, state, source string
		size                int64
		want                codes.Code
		ready               bool
	}{
		{"different-source", "BackedUp", "vol-2", 4294967296, codes.AlreadyExists, false},
		{"failed", "Error", "vol-1", 4294967296, codes.FailedPrecondition, false},
		{"destroyed", "Destroyed", "vol-1", 4294967296, codes.FailedPrecondition, false},
		{"unproven-state", "", "vol-1", 4294967296, codes.FailedPrecondition, false},
		{"missing-size", "BackedUp", "vol-1", 0, codes.FailedPrecondition, false},
		{"pending", "BackingUp", "vol-1", 4294967296, codes.OK, false},
		{"primary-ready", "CreatedOnPrimary", "vol-1", 4294967296, codes.OK, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			connector := &snapshotConnector{snapshot: &cloud.Snapshot{ID: "snap-1", VolumeID: tc.source, Size: tc.size, State: tc.state, CreatedAt: "2026-10-06T18:10:42+0000"}}
			result, err := NewControllerServer(connector).CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{Name: "snapshot-pvc", SourceVolumeId: "vol-1"})
			if status.Code(err) != tc.want {
				t.Fatalf("wanted %v got %v", tc.want, err)
			}
			if err == nil && result.Snapshot.ReadyToUse != tc.ready {
				t.Fatalf("false readiness: %+v", result)
			}
			if connector.creates != 0 || connector.volumeLookups != 0 {
				t.Fatal("existing receipt triggered a mutation or source lookup")
			}
		})
	}
}
func TestSnapshotLookupFailureDoesNotCreate(t *testing.T) {
	connector := &snapshotConnector{lookupErr: errors.New("backend unavailable")}
	_, err := NewControllerServer(connector).CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{Name: "snapshot-pvc", SourceVolumeId: "vol-1"})
	if status.Code(err) != codes.Internal || connector.creates != 0 || connector.volumeLookups != 0 {
		t.Fatal("failed lookup triggered mutation")
	}
}
