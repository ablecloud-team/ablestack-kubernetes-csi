# ABLESTACK Kubernetes CSI

Mold HMAC-SHA256 API 인증을 사용하는 Kubernetes CSI 드라이버입니다. CloudStack CSI의 `main` commit `206ccb52ea976350173ebfcbccb542445996f2de`(SDK v2.19.1 업데이트 포함)을 기준으로 하며, 내부 Mold SDK v2.19.2-mold-test.2를 사용합니다. 원본 설명과 지원 기능은 [README.cloudstack.md](README.cloudstack.md)에 보존합니다.

## 소스 사용

이 저장소를 클론하거나 개인 계정으로 fork한 뒤 아래 명령을 실행합니다. `OWNER`에는 사용하려는 저장소의 소유자를 입력합니다. upstream 조직에 승격되면 `ablecloud-team`을, Origin 시험에서는 해당 fork 소유자를 사용합니다. 기본 브랜치는 `main`이며 기능 개발은 별도 브랜치에서 수행합니다.

```bash
git clone https://github.com/OWNER/ablestack-kubernetes-csi.git
cd ablestack-kubernetes-csi
git switch -c feature/my-csi-change
go test ./...
make build
```

Go 모듈 경로는 `github.com/ablecloud-team/ablestack-kubernetes-csi`로 유지합니다. Mold SDK upstream PR 승격 전에는 `go.mod`의 고정 Origin 시험 tag replace를 사용합니다. 승격 후에는 동일한 source SHA를 확인하고 upstream의 정식 버전으로 교체합니다. Apache SDK/SHA1로 자동 대체하지 않습니다.

## Actions와 이미지

`CSI Build and License`는 branch PR 및 수동 실행에서 전체 Go 시험, 라이선스 원문 유지 검사, linux/amd64 드라이버·StorageClass syncer 바이너리 생성을 수행합니다. 수동 실행에서 publish를 선택한 경우 해당 저장소 소유자의 GHCR로 드라이버 이미지를 게시합니다. 배포에는 반환된 `sha256` digest를 고정해서 사용합니다. `main`/`latest` 이미지 태그로 시험 결과를 대신하지 않습니다.

```bash
gh workflow run csi-build.yml --ref YOUR_BRANCH -f publish=false
gh run list --workflow csi-build.yml
```

## 시험과 배포 조건

현재는 #1280의 Local/Origin qualification 단계입니다. Kubernetes 1.34/1.35/1.36/1.37 DEV의 CSI 지원이 검증됐다는 의미는 아닙니다. 검증 완료 profile만 ISO 선택 항목과 연결하고, 1.37은 개발 후보로 구분합니다.

- Kubernetes CCM과 동일한 `kube-system/cloudstack-secret` 형식과 계정/프로젝트 범위를 사용합니다. 자격증명은 Git·이미지·ISO에 넣지 않습니다.
- `deploy/k8s`는 원본 참고 manifest입니다. 내부 드라이버와 sidecar digest를 고정한 시험 profile을 만들고 실제 imageID 및 바이너리 source를 확인한 뒤 사용합니다. 원본 manifest의 외부 `main` 이미지를 그대로 설치하지 않습니다.
- GFS2 시험에서는 Primary pool 태그에 맞는 전용 shared/custom disk offering을 StorageClass에 지정합니다. CLVM/CLVM_NG에 배치하지 않습니다.
- PVC 생성/attach/mount, 노드 이동, 확장, Delete/Retain, snapshot/restore와 실제 Cloud volume·GFS2 backing file·데이터 checksum을 함께 확인합니다.
- 장기 시험 클러스터는 보존하고 CSI 시험은 별도 클러스터에 한정합니다.

## 라이선스

Apache-2.0. 기존 저작권/라이선스 고지와 원본 Git 이력을 보존합니다. 전체 Cloud 통합 빌드와 공식 이미지/ISO Release 승격은 최종 Release 단계에서 별도로 수행합니다.

### Mold snapshot 복원 재시도

CSI용 API 키에는 snapshot API와 함께 `createTags` 권한이 필요합니다. 복원이 완료되면 Volume에 `mold.csi.snapshot-id` 태그로 원본 snapshot UUID를 저장합니다. 재시도는 같은 snapshot 영수증과 `Ready` 상태를 확인한 후 ContentSource를 다시 반환합니다. `Creating`, `Destroy` 또는 원본 영수증이 없는 볼륨을 성공으로 처리하지 않습니다. `Allocated` 상태는 snapshot을 사용하지 않은 빈 볼륨 생성에만 허용합니다. 기존 실패 볼륨은 운영자가 원인과 소유권을 확인하여 정리해야 합니다.

Snapshot 조회 경로는 API의 `virtualsize`와 생성 시각을 유지합니다. 이 동작은 Mold의 파일 기반 KVM snapshot 저장소 경로 수정(#1283)과 함께 검증해야 합니다.
