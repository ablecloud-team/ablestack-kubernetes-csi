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

package mount

import (
	"context"
	kmount "k8s.io/mount-utils"
	kexec "k8s.io/utils/exec"
	fakeexec "k8s.io/utils/exec/testing"
	"testing"
)

func TestVerifyDeviceRequiresVolumeIdentity(t *testing.T) {
	const volume = "90fbba30-99bf-4405-89e1-66980ae8d13d"
	tests := []struct {
		name, properties string
		want             bool
	}{
		{"requested KVM disk", "ID_SERIAL_SHORT=90fbba3099bf440589e1", true},
		{"previous PVC still detaching", "ID_SERIAL_SHORT=f8a4f70a40b24dcf8bde", false},
		{"readable disk without serial", "ID_MODEL=QEMU_HARDDISK", false},
		{"full UUID", "ID_SERIAL_SHORT=90FBBA30-99BF-4405-89E1-66980AE8D13D", true},
		{"QEMU fallback", "ID_SERIAL=0QEMU_QEMU_HARDDISK_90fbba3099bf440589e1", true},
		{"substring is not identity", "ID_SERIAL=foreign_90fbba3099bf440589e1", false},
		{"short property mismatch wins", "ID_SERIAL_SHORT=f8a4f70a40b24dcf8bde\nID_SERIAL=0QEMU_QEMU_HARDDISK_90fbba3099bf440589e1", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeexec.FakeExec{CommandScript: []fakeexec.FakeCommandAction{
				func(cmd string, args ...string) kexec.Cmd {
					if cmd != "blockdev" {
						t.Fatalf("unexpected command %s", cmd)
					}
					return fakeexec.InitFakeCmd(&fakeexec.FakeCmd{OutputScript: []fakeexec.FakeAction{func() ([]byte, []byte, error) { return []byte("4294967296"), nil, nil }}}, cmd, args...)
				},
				func(cmd string, args ...string) kexec.Cmd {
					if cmd != "udevadm" {
						t.Fatalf("unexpected command %s", cmd)
					}
					return fakeexec.InitFakeCmd(&fakeexec.FakeCmd{OutputScript: []fakeexec.FakeAction{func() ([]byte, []byte, error) { return []byte(tc.properties), nil, nil }}}, cmd, args...)
				},
			}}
			m := &mounter{SafeFormatAndMount: &kmount.SafeFormatAndMount{Interface: kmount.NewFakeMounter(nil), Exec: f}}
			if got := m.verifyDevice(context.Background(), "/dev/sdb", volume); got != tc.want {
				t.Fatalf("verifyDevice = %v, want %v", got, tc.want)
			}
		})
	}
	if deviceSerialMatches(map[string]string{"ID_SERIAL_SHORT": "invalid"}, "invalid") {
		t.Fatal("invalid volume identity accepted")
	}
}
