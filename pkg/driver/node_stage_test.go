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
	"os"
	"path/filepath"
	"testing"

	"github.com/ablecloud-team/ablestack-kubernetes-csi/pkg/mount"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stageTestMounter struct {
	mount.Interface
	device, source                string
	references, formats, unstages int
}

func (m *stageTestMounter) GetDevicePath(context.Context, string) (string, error) {
	return m.source, nil
}
func (m *stageTestMounter) GetDeviceName(string) (string, int, error) {
	return m.device, m.references, nil
}
func (m *stageTestMounter) FormatAndMount(string, string, string, []string) error {
	m.formats++
	return nil
}
func (m *stageTestMounter) Unstage(string) error { m.unstages++; return nil }

func TestNodeStageRejectsStaleDeviceWithoutFormatting(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "sdc")
	if err := os.WriteFile(source, nil, 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "by-id")
	if err := os.Symlink(source, alias); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, mounted, source string
		refs, formats         int
		code                  codes.Code
	}{
		{"detached old device", "/dev/sdb", "/dev/sdc", 2, 0, codes.FailedPrecondition},
		{"occupied target without identity", "", "/dev/sdc", 1, 0, codes.FailedPrecondition},
		{"missing different alias", filepath.Join(dir, "missing"), source, 1, 0, codes.FailedPrecondition},
		{"same device idempotent", "/dev/sdc", "/dev/sdc", 1, 0, codes.OK},
		{"stable alias idempotent", source, alias, 1, 0, codes.OK},
		{"unstaged target", "", source, 0, 1, codes.OK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &stageTestMounter{Interface: mount.NewFake(), device: tc.mounted, source: tc.source, references: tc.refs}
			ns := NewNodeServer(nil, m, &Options{})
			_, err := ns.NodeStageVolume(context.Background(), &csi.NodeStageVolumeRequest{
				VolumeId: "90fbba30-99bf-4405-89e1-66980ae8d13d", StagingTargetPath: dir,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{FsType: "ext4"}},
					AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
				},
			})
			if status.Code(err) != tc.code {
				t.Fatalf("code = %v, want %v: %v", status.Code(err), tc.code, err)
			}
			if m.formats != tc.formats {
				t.Fatalf("FormatAndMount calls = %d, want %d", m.formats, tc.formats)
			}
			if m.unstages != 0 {
				t.Fatal("stage must not unmount an occupied target")
			}
		})
	}
}
