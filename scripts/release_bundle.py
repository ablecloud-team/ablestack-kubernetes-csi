#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
# Licensed under the Apache License, Version 2.0.
# https://www.apache.org/licenses/LICENSE-2.0
"""Package the exact Actions build and a digest-locked Mold GFS2 profile."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess

SDK = "github.com/ablecloud-team/ablestack-mold-go/v2"
SDK_VERSION = "v2.19.2-mold.1"
SDK_SOURCE = "06c94ac6637fe03b795cef10ba1afde1d1e8ef8c"

def digest(path):
    with path.open('rb') as stream:
        checksum = hashlib.sha256()
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            checksum.update(chunk)
        return checksum.hexdigest()

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--image', required=True)
    parser.add_argument('--source', required=True)
    parser.add_argument('--repository', required=True)
    parser.add_argument('--run-url', required=True)
    args = parser.parse_args()
    if not re.fullmatch(r'ghcr\.io/[a-z0-9_-]+/ablestack-kubernetes-csi@sha256:[0-9a-f]{64}', args.image):
        raise SystemExit('A digest-locked internal CSI image is required')
    if not re.fullmatch(r'[0-9a-f]{40}', args.source):
        raise SystemExit('A complete source commit is required')
    if subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip() != args.source:
        raise SystemExit('Source commit does not match checkout')
    module = json.loads(subprocess.check_output(['go', 'list', '-m', '-json', SDK], text=True))
    if module['Version'] != SDK_VERSION or module.get('Replace'):
        raise SystemExit('Official Mold SDK without a candidate replacement is required')
    modules = subprocess.check_output(['go', 'list', '-m', 'all'], text=True)
    if re.search(r'github.com/(apache/cloudstack-go|cloudstack/cloudstack-csi-driver|dhslove/ablestack-mold-go)|mold-test', modules):
        raise SystemExit('Unqualified SDK or original driver dependency')
    out = args.output
    out.mkdir(parents=True, exist_ok=True)
    root = Path('deploy/profiles/mold-gfs2-amd64')
    profile = json.loads((root/'profile.json').read_text())
    for name, expected in profile['files'].items():
        if digest(root/name) != expected:
            raise SystemExit('Source profile checksum mismatch: ' + name)
    old = profile['driverImage']
    manifest = (root/'manifest.yaml').read_text()
    if manifest.count(old) != 2:
        raise SystemExit('Expected both controller and node driver image locks')
    manifest = manifest.replace(old, args.image)
    (out/'manifest.yaml').write_text(manifest)
    shutil.copy2(root/'snapshot-crds.yaml', out/'snapshot-crds.yaml')
    profile.update(driverImage=args.image, binarySource=args.source,
                   sdk=dict(module=SDK, version=SDK_VERSION, source=SDK_SOURCE),
                   qualification='europa-gfs2-representative-runtime-and-official-sdk-build',
                   sourceRepository=args.repository, buildRun=args.run_url)
    profile['images'] = {key: args.image if value == old else value for key, value in profile['images'].items()}
    profile['files'] = {name: digest(out/name) for name in profile['files']}
    (out/'profile.json').write_text(json.dumps(profile, indent=2)+'\n')
    if len(profile['images']) != 8 or set(re.findall(r'^\s*image:\s*(\S+)', manifest, re.M)) != set(profile['images'].values()):
        raise SystemExit('Manifest image locks do not match the eight profile images')
    if re.search(r'^kind:\s*Secret\s*$', manifest, re.M):
        raise SystemExit('Release profile must not contain credentials')
    for command in ['cloudstack-csi-driver', 'cloudstack-csi-sc-syncer']:
        binary = Path('bin')/command
        if binary.read_bytes()[:4] != b'\x7fELF':
            raise SystemExit('Expected Linux ELF binary: '+command)
        build_info = subprocess.check_output(['go', 'version', '-m', str(binary)], text=True)
        if f'{SDK}\t{SDK_VERSION}' not in build_info or 'mold-test' in build_info or '\n\t=>' in build_info:
            raise SystemExit('Binary does not contain the official SDK: '+command)
        shutil.copy2(binary, out/(command+'-linux-amd64'))
        (out/(command+'-buildinfo.txt')).write_text(build_info)
    version = json.loads(subprocess.check_output(['bin/cloudstack-csi-driver', '--version'], text=True))
    (out/'driver-version.json').write_text(json.dumps(version, indent=2)+'\n')
    if args.source not in json.dumps(version):
        raise SystemExit('Driver version must identify the exact build commit')
    (out/'go-modules.txt').write_text(modules)
    (out/'image-digest.txt').write_text(args.image+'\n')
    for name in ['LICENSE', 'NOTICE']:
        shutil.copy2(name, out/name)
    provenance = dict(schemaVersion=1, sourceRepository=args.repository,
                      sourceSHA=args.source, buildRun=args.run_url, architecture='linux/amd64',
                      apiSignature='HmacSHA256', driverImage=args.image, sdk=profile['sdk'],
                      qualification=profile['qualification'],
                      qualificationEvidence='https://github.com/ablecloud-team/ablestack-cloud/blob/c169d9a203f49ce07e038297873bc3c24cd8ffb4/docs/operations/kubernetes-lifecycle/qualification-20261007.md',
                      qualificationScope='Europa 31 KVM/GFS2 Primary representative CSI runtime: 1.34.12, 1.35.9, 1.36.5, 1.37.1. Official SDK content equals the tested SDK; this rebuild is not a new six-patch runtime deployment.')
    (out/'provenance.json').write_text(json.dumps(provenance, indent=2)+'\n')
    (out/'SHA256SUMS').write_text(''.join(f'{digest(p)}  {p.name}\n' for p in sorted(out.iterdir()) if p.is_file() and p.name != 'SHA256SUMS'))
    print('Official SDK binaries, eight digest locks, profile and release checksums verified')

if __name__ == '__main__':
    main()
