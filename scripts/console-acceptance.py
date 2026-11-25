#!/usr/bin/env python3
"""Disposable console/API acceptance; never uses existing demo nodes or data.

Build bin/raftkv first. This checks the API behind the UI, not browser rendering.
"""
import argparse
import datetime
import json
import pathlib
import socket
import subprocess
import tempfile
import time
import urllib.request
import urllib.error


def run(binary, evidence):
    with tempfile.TemporaryDirectory(prefix="raftkv-console-acceptance-") as tmp:
        root = pathlib.Path(tmp)
        processes, logs, reservations = [], [], []

        def port():
            s = socket.socket()
            s.bind(('127.0.0.1', 0))
            reservations.append(s)
            return s.getsockname()[1]

        peers = [dict(id=f'accept{i}', http_address=f'http://127.0.0.1:{port()}',
                      grpc_address=f'127.0.0.1:{port()}') for i in range(3)]
        http, grpc = port(), port()
        base = f'http://127.0.0.1:{http}'
        config = root / 'cluster.json'
        config.write_text(json.dumps({'peers': peers}))
        for s in reservations:
            s.close()

        def start(args, name):
            log = open(root / f'{name}.log', 'ab')
            logs.append(log)
            p = subprocess.Popen([str(binary), *args], stdout=log, stderr=log)
            processes.append(p)
            return p

        def stop(p):
            if p.poll() is None:
                p.terminate()
            try:
                p.wait(timeout=10)
            except subprocess.TimeoutExpired:
                p.kill()
                p.wait()
                raise RuntimeError('Process did not shut down gracefully')

        def call(path, payload=None):
            data = None if payload is None else json.dumps(payload).encode()
            req = urllib.request.Request(base + path, data=data,
                                         headers={'Content-Type': 'application/json'})
            with urllib.request.urlopen(req, timeout=12) as r:
                return json.load(r)

        def wait(check, label):
            deadline = time.monotonic() + 30
            last = None
            while time.monotonic() < deadline:
                try:
                    result = check()
                    if result:
                        return result
                except (OSError, ValueError) as e:
                    last = str(e)
                time.sleep(.15)
            raise RuntimeError(f'{label} timed out: {last}')

        def leader():
            rows = call('/demo/api/cluster/status')
            leaders = [r['status'] for r in rows if r.get('status', {}).get('role') == 'leader']
            return leaders[0] if len(leaders) == 1 and not leaders[0]['membership_pending'] else None

        def command(op, **kwargs):
            s = wait(leader, 'stable leader')
            return call('/demo/api/cluster/command', dict(node=s['id'], operation=op, **kwargs))

        console_args = ['--demo', '--node-id', peers[0]['id'], '--demo-cluster-config', str(config), '--http-addr', f'127.0.0.1:{http}',
                        '--grpc-addr', f'127.0.0.1:{grpc}', '--demo-node-seed', str(config),
                        '--demo-node-dir', str(root / 'managed')]
        originals = {}
        node_args = {}
        try:
            for p in peers:
                node_args[p['id']] = ['--node-id', p['id'], '--cluster-config', str(config), '--data-dir', str(root / p['id']),
                       '--http-addr', p['http_address'].removeprefix('http://'), '--grpc-addr', p['grpc_address']]
                originals[p['id']] = start(node_args[p['id']], p['id'])
            console = start(console_args, 'console')
            wait(leader, 'initial election')
            assert call('/demo/api/cluster/capabilities')['local_processes']
            with urllib.request.urlopen(base + '/demo/', timeout=5) as r:
                assert 'auto-membership' in r.read().decode()
            command('PUT', key='acceptance', value='retained')
            assert command('GET', key='acceptance')['value'] == 'retained'
            evidence.append('console page, process capability and replicated PUT/GET')
            peer = call('/demo/api/cluster/process', {'action': 'create'})
            command('join', member=peer['id'])

            def caught_up():
                s = leader()
                rows = call('/demo/api/cluster/status')
                row = next((r for r in rows if r['peer']['id'] == peer['id']), {})
                return s and row.get('status', {}).get('last_applied', -1) >= s['commit_index']

            wait(caught_up, 'learner catch-up')
            command('promote', member=peer['id'])
            wait(lambda: (s := leader()) and len(s['peers']) == 4 and
                 any(p['id'] == peer['id'] and not p['learner'] for p in s['peers']), 'promotion')
            evidence.append('automatic ports, learner join, catch-up and stable four-voter membership')
            stop(console)
            console = start(console_args, 'console')
            wait(caught_up, 'managed restart')
            row = next(r for r in call('/demo/api/cluster/status') if r['peer']['id'] == peer['id'])
            assert row['peer'] == peer and row['managed']
            assert command('GET', key='acceptance')['value'] == 'retained'
            evidence.append('console restart restores managed identity, ports and committed value')
            command('remove', member=peer['id'])
            wait(lambda: (s := leader()) and len(s['peers']) == 3 and
                 all(p['id'] != peer['id'] for p in s['peers']), 'removal')
            assert command('GET', key='acceptance')['value'] == 'retained'
            evidence.append('stable removal and retained consensus read')
            active = wait(leader, 'leader before quorum loss')
            followers = [p['id'] for p in peers if p['id'] != active['id']]
            for identity in followers:
                originals[identity].kill()
                originals[identity].wait(timeout=5)
            before = wait(leader, 'surviving leader')['commit_index']
            for operation in ('GET', 'PUT'):
                payload = dict(node=active['id'], operation=operation, key='minority-write', value='uncertain')
                try:
                    call('/demo/api/cluster/command', payload)
                except urllib.error.HTTPError as e:
                    if e.code not in (503, 504):
                        raise
                else:
                    raise AssertionError(f'{operation} succeeded without quorum')
            assert leader()['commit_index'] == before, 'commit advanced without quorum'
            evidence.append('two follower SIGKILLs: GET and PUT unavailable, commit index unchanged')
            for identity in followers:
                originals[identity] = start(node_args[identity], identity)
            # Role reports may precede recovered quorum. Retry only the
            # verification read; do not replay an unknown-outcome mutation.
            wait(lambda: command('GET', key='acceptance')['value'] == 'retained',
                 'verified consensus read after quorum recovery')
            # The minority PUT timed out with unknown outcome. Overwrite it only
            # after recovery, then verify a new acknowledged value.
            command('PUT', key='minority-write', value='recovered')
            assert command('GET', key='minority-write')['value'] == 'recovered'
            evidence.append('quorum recovery retains acknowledged data and accepts a new write/read')
        except Exception:
            for path in root.rglob('*.log'):
                print(path.name, path.read_text(errors='replace')[-3000:])
            raise
        finally:
            for p in reversed(processes):
                try:
                    stop(p)
                except RuntimeError as error:
                    print(f'Cleanup: {error}')
            for log in logs:
                log.close()
    return evidence


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=pathlib.Path, default=pathlib.Path('bin/raftkv'))
    parser.add_argument('--output', type=pathlib.Path, required=True)
    args = parser.parse_args()
    report = {'timestamp_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
              'scope': 'Disposable local processes; console API, not browser or Docker acceptance',
              'passed': []}
    try:
        run(args.binary.resolve(), report['passed'])
        report['result'] = 'passed'
    except Exception as error:
        report['result'] = 'failed'
        report['error'] = f'{type(error).__name__}: {error}'
        raise
    finally:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))
