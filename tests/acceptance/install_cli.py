#!/usr/bin/env python3
"""Install pinned CLI test binaries, verifying official SHA256SUMS."""
import hashlib
from pathlib import Path
import sys
import urllib.request
import zipfile
import io

cli, destination = sys.argv[1:]
if cli == 'terraform':
    version = '1.15.9'
    base = f'https://releases.hashicorp.com/terraform/{version}'
    sums = f'terraform_{version}_SHA256SUMS'
elif cli == 'tofu':
    version = '1.13.2'
    base = f'https://github.com/opentofu/opentofu/releases/download/v{version}'
    sums = f'tofu_{version}_SHA256SUMS'
else:
    sys.exit('Expected terraform or tofu')
name = f'{cli}_{version}_linux_amd64.zip'
def download(name):
    with urllib.request.urlopen(base + '/' + name, timeout=60) as response:
        return response.read()
checksums = download(sums).decode()
expected = next(line.split()[0] for line in checksums.splitlines() if line.split()[-1].lstrip('*') == name)
data = download(name)
if hashlib.sha256(data).hexdigest() != expected:
    sys.exit('CLI checksum mismatch')
path = Path(destination)
path.mkdir(parents=True, exist_ok=True)
with zipfile.ZipFile(io.BytesIO(data)) as archive:
    binary = path / cli
    binary.write_bytes(archive.read(cli))
binary.chmod(0o755)
