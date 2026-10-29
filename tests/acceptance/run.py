#!/usr/bin/env python3
"""Run opt-in acceptance tests against a fresh server, never a supplied URL."""
import base64
import hashlib
import json
import os
from pathlib import Path
import secrets
import socket
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.request
from examples import verify_examples

IMAGE = 'ghcr.io/linuxserver/qbittorrent@sha256:75ecbb1c89ab7b0a21c4bde40b4820e4f1649d2d56d83b91913422155e8d2a5f'
ROOT = Path(__file__).resolve().parents[2]

def run(*args, **kwargs):
    return subprocess.run(args, check=True, **kwargs)

def main():
    binary = os.environ.get('TF_ACC_TERRAFORM_PATH') or shutil.which('terraform') or shutil.which('tofu')
    if not binary:
        sys.exit('Install Terraform or OpenTofu, or set TF_ACC_TERRAFORM_PATH')
    name = 'qbt-provider-acc-' + secrets.token_hex(5)
    network = name + '-test'
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = str(listener.getsockname()[1])
    key = 'qbt_' + secrets.token_hex(14)
    password = secrets.token_urlsafe(24)
    for secret in (key, password):
        if os.environ.get('GITHUB_ACTIONS'):
            print('::add-mask::' + secret, flush=True)
    with tempfile.TemporaryDirectory(prefix='qbt-provider-acc-') as tmp:
        config = Path(tmp) / 'qBittorrent'
        config.mkdir()
        salt = secrets.token_bytes(16)
        digest = hashlib.pbkdf2_hmac('sha512', password.encode(), salt, 100000)
        encoded = base64.b64encode(salt).decode() + ':' + base64.b64encode(digest).decode()
        (config / 'qBittorrent.conf').write_text(
            '[LegalNotice]\nAccepted=true\n[WebUI]\n'
            'Username=admin\nPassword_PBKDF2="' + encoded + '"\n'
            'APIKey=' + key + '\n'
            'LocalHostAuth=true\nAuthSubnetWhitelistEnabled=false\n'
            'CSRFProtection=true\nHostHeaderValidation=true\n'
            'ServerDomains=127.0.0.1\n'
            '[BitTorrent]\nSession\\DHTEnabled=false\nSession\\PeXEnabled=false\n'
            'Session\\LSDEnabled=false\n[Network]\nPortForwardingEnabled=false\n'
        )
        run('docker', 'network', 'create', network, stdout=subprocess.DEVNULL)
        try:
            # Only the WebUI is published, on loopback. No BitTorrent port is published.
            run('docker', 'run', '-d', '--name', name, '--network', network,
                '-p', f'127.0.0.1:{port}:{port}', '-e', 'WEBUI_PORT=' + port, '-e', 'PUID=' + str(os.getuid()),
                '-e', 'PGID=' + str(os.getgid()), '-v', tmp + ':/config', IMAGE,
                stdout=subprocess.DEVNULL)
            url = 'http://127.0.0.1:' + port
            def get(endpoint):
                req = urllib.request.Request(url + '/api/v2/' + endpoint, headers={'Authorization': 'Bearer ' + key})
                with urllib.request.urlopen(req, timeout=2) as response:
                    return response.read().decode()
            for attempt in range(90):
                try:
                    app, api = get('app/version'), get('app/webapiVersion')
                    break
                except Exception as error:
                    if attempt in (5, 30, 60):
                        print("Waiting for disposable WebUI:", type(error).__name__, getattr(error, "code", ""), flush=True)
                    time.sleep(1)
            else:
                sys.exit('Disposable qBittorrent failed to start (logs withheld to protect credentials)')
            if (app, api) != ('v5.3.0', '2.16.2'):
                sys.exit('Container version mismatch: ' + repr((app, api)))
            preferences = json.loads(get('app/preferences'))
            if any(preferences.get(k) is not False for k in ('dht', 'pex', 'lsd', 'upnp')):
                sys.exit('Disposable server discovery/port-forwarding flags must be disabled')
            print('Image:', IMAGE, '\nApplication:', app, '\nWeb API:', api, flush=True)
            run(binary, 'version')
            env = os.environ.copy()
            for k in ('QBITTORRENT_URL', 'QBITTORRENT_API_KEY', 'QBITTORRENT_USERNAME', 'QBITTORRENT_PASSWORD'):
                env.pop(k, None)
            env.update(TF_ACC='1', TF_ACC_TERRAFORM_PATH=binary, QBT_ACC_URL=url,
                       QBT_ACC_API_KEY=key, QBT_ACC_USERNAME='admin', QBT_ACC_PASSWORD=password,
                       QBT_ACC_CONTAINER=name)
            # Unit tests always run separately. Acceptance tests only accept QBT_ACC_*.
            run('go', 'test', './internal/provider', '-run', 'TestAcc', '-count=1', '-timeout', '20m', '-v', cwd=ROOT, env=env)
            verify_examples(binary, env, ROOT, tmp)
        finally:
            subprocess.run(['docker', 'rm', '-f', name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            subprocess.run(['docker', 'network', 'rm', network], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

if __name__ == '__main__':
    main()
