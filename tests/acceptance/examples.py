"""Validate and plan the shipped examples against the disposable server."""
import os
from pathlib import Path
import subprocess

def verify_examples(cli, environment, root, scratch):
    work = Path(scratch) / 'examples'
    work.mkdir()
    goos = subprocess.check_output(['go', 'env', 'GOOS'], text=True).strip()
    goarch = subprocess.check_output(['go', 'env', 'GOARCH'], text=True).strip()
    mirror = Path(scratch) / 'mirror'
    # OpenTofu normalizes shorthand sources to registry.opentofu.org.
    host = 'registry.opentofu.org' if Path(cli).name == 'tofu' else 'registry.terraform.io'
    package = mirror / (host + '/m11s-io/qbittorrent/0.1.0') / (goos + '_' + goarch)
    package.mkdir(parents=True)
    subprocess.run(['go', 'build', '-o', str(package / 'terraform-provider-qbittorrent_v0.1.0'), '.'], cwd=root, check=True)
    config = Path(scratch) / 'cli.tfrc'
    config.write_text('provider_installation { filesystem_mirror { path = "' + str(mirror) + '" } }\n')
    example_files = [
        'examples/provider/provider.tf',
        'examples/resources/qbittorrent_preferences/resource.tf',
        'examples/resources/qbittorrent_category/resource.tf',
        'examples/resources/qbittorrent_webui_preferences/resource.tf',
        'examples/data-sources/qbittorrent_preferences/data-source.tf',
    ]
    source = '\n'.join((root / p).read_text() for p in example_files)
    source = source.replace('https://qbittorrent.example.com', environment['QBT_ACC_URL'])
    (work / 'main.tf').write_text(source)
    env = environment.copy()
    env.update(TF_CLI_CONFIG_FILE=str(config), QBITTORRENT_URL=env['QBT_ACC_URL'],
               QBITTORRENT_API_KEY=env['QBT_ACC_API_KEY'], QBITTORRENT_USERNAME='', QBITTORRENT_PASSWORD='',
               CHECKPOINT_DISABLE='1')
    env.pop('TF_REATTACH_PROVIDERS', None)
    for args in [('init', '-backend=false', '-input=false', '-no-color'),
                 ('validate', '-no-color'), ('plan', '-input=false', '-lock=false', '-no-color')]:
        subprocess.run([cli, *args], cwd=work, env=env, check=True)
