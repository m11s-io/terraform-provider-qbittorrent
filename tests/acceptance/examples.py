"""Validate and plan the shipped examples against the disposable server."""
import os
import secrets
import shutil
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
    alternate_host = 'registry.terraform.io' if host == 'registry.opentofu.org' else 'registry.opentofu.org'
    alternate = mirror / (alternate_host + '/m11s-io/qbittorrent/0.1.0') / (goos + '_' + goarch)
    alternate.mkdir(parents=True)
    shutil.copy2(package / 'terraform-provider-qbittorrent_v0.1.0', alternate)
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
    env = environment.copy()
    env.update(TF_CLI_CONFIG_FILE=str(config), QBITTORRENT_URL=env['QBT_ACC_URL'],
               QBITTORRENT_API_KEY=env['QBT_ACC_API_KEY'], QBITTORRENT_USERNAME='', QBITTORRENT_PASSWORD='',
               CHECKPOINT_DISABLE='1')
    env.pop('TF_REATTACH_PROVIDERS', None)
    # Example passwords are ephemeral inputs. Plans never apply them to the
    # server, and random values avoid borrowing any user credentials.
    for variable in ('smtp_password', 'proxy_password', 'dyndns_password'):
        value = secrets.token_urlsafe(24)
        if os.environ.get('GITHUB_ACTIONS'):
            print('::add-mask::' + value, flush=True)
        env['TF_VAR_' + variable] = value
    # Each coverage/maintenance example is independent partial ownership.
    # Validate and plan it separately; never apply example maintenance changes.
    groups = [("existing", source)]
    baseline = {root / p for p in example_files}
    additions = sorted(set((root / 'examples/coverage').glob('*.tf')) |
                       set((root / 'examples/maintenance').glob('*.tf')) |
                       (set((root / 'examples/resources').glob('*/*.tf')) - baseline))
    provider = (root / 'examples/provider/provider.tf').read_text().replace(
        'https://qbittorrent.example.com', environment['QBT_ACC_URL'])
    for number, example in enumerate(additions):
        contents = example.read_text()
        groups.append((f"example-{number}", contents if 'provider "qbittorrent"' in contents else provider + "\n" + contents))
    for name, contents in groups:
        directory = work / name
        directory.mkdir()
        (directory / 'main.tf').write_text(contents)
        example_env = env.copy()
        if 'variable "provider_url"' in contents:
            example_env.update(QBITTORRENT_API_KEY='',
                               TF_VAR_provider_url=environment['QBT_ACC_URL'],
                               TF_VAR_provider_username=environment['QBT_ACC_USERNAME'],
                               TF_VAR_provider_password=environment['QBT_ACC_PASSWORD'],
                               TF_VAR_target_webui_url='http://127.0.0.1:18081',
                               TF_VAR_target_webui_username='fixture-operator',
                               TF_VAR_target_webui_password=secrets.token_urlsafe(24))
        print('Checking example:', name, flush=True)
        for args in [('init', '-backend=false', '-input=false', '-no-color'),
                     ('validate', '-no-color'), ('plan', '-input=false', '-lock=false', '-no-color')]:
            subprocess.run([cli, *args], cwd=directory, env=example_env, check=True)
