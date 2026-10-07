<!--
SPDX-License-Identifier: Apache-2.0
Licensed under the Apache License, Version 2.0.
https://www.apache.org/licenses/LICENSE-2.0
-->
# ABLESTACK Kubernetes CSI

Mold HMAC-SHA256 API 인증을 사용하는 Kubernetes CSI 드라이버입니다. CloudStack CSI의 `main` commit `206ccb52ea976350173ebfcbccb542445996f2de`(SDK v2.19.1 업데이트 포함)을 기준으로 하며, 정식 [Mold SDK v2.19.2-mold.1](https://github.com/ablecloud-team/ablestack-mold-go/releases/tag/v2.19.2-mold.1)를 사용합니다. 원본 설명과 지원 기능은 [README.cloudstack.md](README.cloudstack.md)에 보존합니다.

## 소스 사용

이 저장소를 클론하거나 개인 계정으로 fork한 뒤 아래 명령을 실행합니다. `OWNER`에는 사용하려는 저장소의 소유자를 입력합니다. 공식 upstream을 사용할 때는 `ablecloud-team`을, 개인 fork에서는 해당 소유자를 사용합니다. 기본 브랜치는 `main`이며 기능 개발은 별도 브랜치에서 수행합니다.

```bash
git clone https://github.com/OWNER/ablestack-kubernetes-csi.git
cd ablestack-kubernetes-csi
git switch -c feature/my-csi-change
go test ./...
make build
```

Go 모듈 경로는 `github.com/ablecloud-team/ablestack-kubernetes-csi`로 유지합니다. 정식 Mold SDK를 직접 사용하며 시험 fork로 향하는 `replace`는 없습니다. SDK의 HMAC-SHA256 계약을 유지합니다. Apache SDK/SHA1로 자동 대체하지 않습니다.

## Actions와 이미지

`CSI Build and License`는 PR 및 수동 실행에서 전체 Go race 시험, 내부 드라이버 CSI sanity 시험, 공식 SDK 의존성 검사, 라이선스 고지 검사, linux/amd64 드라이버·StorageClass syncer 바이너리 생성을 수행합니다. 전체 저장소 Apache RAT 검사는 별도 `License Check`에서 수행합니다. 수동 실행에서 publish를 선택하면 해당 저장소 소유자의 GHCR에 드라이버 이미지를 게시하고 릴리즈 bundle을 Actions 산출물로 생성합니다. bundle에는 이미지 archive와 이미지 내 실제 바이너리, 독립 바이너리 2종, 8개 이미지 digest를 고정한 manifest/CRD/profile, provenance, Go module 목록, LICENSE/NOTICE, SHA256SUMS가 포함됩니다. Actions는 자동으로 GitHub Release를 게시하지 않으며 검증한 bundle을 공식 Release에 그대로 게시합니다. 배포에는 반환된 `sha256` digest를 고정해서 사용합니다. `main`/`latest` 이미지 태그로 시험 결과를 대신하지 않습니다.

```bash
gh workflow run csi-build.yml --ref YOUR_BRANCH -f publish=false
gh run list --workflow csi-build.yml
```

## 시험과 배포 조건

Europa 31의 KVM/GFS2 Primary에서 별도 클러스터의 Kubernetes 1.34.12·1.35.9·1.36.5·1.37.1 조합으로 CSI 수명주기를 검증했습니다. [실환경 판정과 한계](https://github.com/ablecloud-team/ablestack-cloud/blob/c169d9a203f49ce07e038297873bc3c24cd8ffb4/docs/operations/kubernetes-lifecycle/qualification-20261007.md)를 확인하세요. 1.37.1 AutoScaler의 사용자 Mold 프로덕션 판정은 해당 기능의 별도 판정으로 유지합니다. CSI는 위 대표 실환경 검증 범위를 적용합니다. 원본 프로젝트 지원 선언이나 모든 patch 버전의 개별 재시험을 뜻하지 않습니다. 이번 정식 SDK 빌드는 시험 SDK와 동일한 구현 내용 및 내부 드라이버 코드의 연속성을 확인하고 빌드/race/sanity 검증을 추가합니다. 1.34.2·1.34.9의 CSI 개별 실환경 시험과 이 새 바이너리의 전체 버전 재배포는 별도입니다.

- Kubernetes CCM과 동일한 `kube-system/cloudstack-secret` 형식과 계정/프로젝트 범위를 사용합니다. 자격증명은 Git·이미지·ISO에 넣지 않습니다.
- `deploy/k8s`는 원본 참고 manifest입니다. 공식 Release의 내부 드라이버와 sidecar digest를 고정한 profile을 사용하고 실제 imageID 및 바이너리 source를 확인한 뒤 사용합니다. 원본 manifest의 외부 `main` 이미지를 그대로 설치하지 않습니다.
- GFS2 시험에서는 Primary pool 태그에 맞는 전용 shared/custom disk offering을 StorageClass에 지정합니다. CLVM/CLVM_NG에 배치하지 않습니다.
- PVC 생성/attach/mount, 노드 이동, 확장, Delete/Retain, snapshot/restore와 실제 Cloud volume·GFS2 backing file·데이터 checksum을 함께 확인합니다.
- 장기 시험 클러스터는 보존하고 CSI 시험은 별도 클러스터에 한정합니다.

## 릴리즈 사용과 CSI ISO

공식 [Mold CSI 릴리즈](https://github.com/ablecloud-team/ablestack-kubernetes-csi/releases/tag/mold-csi-r1-a135d6fc75fd)에서 `SHA256SUMS`, `profile.json`, `manifest.yaml`, `snapshot-crds.yaml`, `provenance.json`을 받습니다. `cloudstack-csi-*` chart 태그만 받은 것은 내부 SHA256 드라이버 설치가 아닙니다. archive와 Linux 바이너리는 offline 보관 및 진단용입니다. Kubernetes에서는 `profile.json.driverImage`의 digest를 설치합니다. `deploy/profiles/mold-gfs2-amd64`는 이 릴리즈의 manifest/profile을 동일 내용으로 고정합니다. 다른 릴리즈의 파일과 섞지 마세요.

```bash
sha256sum --check SHA256SUMS
kubectl apply -f snapshot-crds.yaml
# CCM의 kube-system/cloudstack-secret과 CSI용 API 권한을 먼저 준비합니다.
kubectl apply -f manifest.yaml
kubectl -n kube-system rollout status deployment/cloudstack-csi-controller
kubectl -n kube-system rollout status daemonset/cloudstack-csi-node
```

Mold를 통한 자동 설치는 동일 Kubernetes patch 버전의 별도 CSI ISO에 이 Release의 3개 profile 파일과 8개 digest 이미지가 포함돼 있어야 합니다. 기본 `mold-cks` ISO는 CSI를 포함하지 않습니다. 클러스터 생성의 고급 모드에서 CSI 활성화를 선택하며, ISO 이름에 `csi`를 붙이는 것만으로 활성화되지 않습니다. 드라이버 Release 게시와 선택형 CSI ISO 게시 상태는 각각 확인하세요.

GFS2 Primary 태그에 연결되는 전용 shared/custom disk offering의 실제 UUID를 지정합니다. 노드 ROOT 디스크와 PVC 데이터 디스크의 offering은 별개입니다. 자격증명·API 키·Secret을 릴리즈나 ISO에 넣지 마세요.

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: mold-gfs2-retain
provisioner: csi.cloudstack.apache.org
parameters:
  csi.cloudstack.apache.org/disk-offering-id: "<GFS2-Primary-shared-custom-disk-offering-UUID>"
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
reclaimPolicy: Retain
```

PVC를 사용하는 Pod를 생성한 뒤 Bound, attach/mount, 실제 파일 쓰기·재시작·노드 이동·확장과 checksum을 확인합니다. `Retain` 데이터는 PV/PVC 삭제 후 운영자가 소유권을 확인해 정리합니다. `Delete`는 실제 Mold 볼륨 삭제까지 수행하므로 보존 데이터에 지정하지 마세요. snapshot 복원은 아래의 권한 및 영수증 조건을 따릅니다.

## 라이선스

Apache-2.0. 기존 저작권/라이선스 고지와 원본 Git 이력을 보존합니다. 공식 CSI 드라이버 Release와 선택형 CSI ISO 게시, 전체 Cloud 릴리즈 통합은 구분합니다.

### Mold snapshot 복원 재시도

CSI용 API 키에는 snapshot API와 함께 `createTags` 권한이 필요합니다. 복원이 완료되면 Volume에 `mold.csi.snapshot-id` 태그로 원본 snapshot UUID를 저장합니다. 재시도는 같은 snapshot 영수증과 `Ready` 상태를 확인한 후 ContentSource를 다시 반환합니다. `Creating`, `Destroy` 또는 원본 영수증이 없는 볼륨을 성공으로 처리하지 않습니다. `Allocated` 상태는 snapshot을 사용하지 않은 빈 볼륨 생성에만 허용합니다. 기존 실패 볼륨은 운영자가 원인과 소유권을 확인하여 정리해야 합니다.

Snapshot 조회 경로는 API의 `virtualsize`와 생성 시각을 유지합니다. 이 동작은 Mold의 파일 기반 KVM snapshot 저장소 경로 수정(#1283)과 함께 검증해야 합니다.

### Volume attachment identity and recovery

The Mold KVM profile selects a block device only when its udev serial exactly matches the requested volume UUID or CloudStack's 20-character KVM serial. A readable disk, mount state, or a serial substring does not establish identity. This applies to direct devices and by-id links, including when a previous PVC's asynchronous detach is still pending.

A staging path occupied by a different or unknown device returns `FailedPrecondition`. The driver does not format over or force-unmount that path. Delete the owning Pod normally, allow kubelet's NodeUnpublish/NodeUnstage and the CSI attachment controller to finish, then retry the workload. Retain PVs and their Mold volumes remain intact. Verify the original file checksum after reattachment; Pod Ready alone does not prove data recovery. Current storage qualification targets Mold KVM with GFS2 Primary; other hypervisor serial formats require separate qualification.
