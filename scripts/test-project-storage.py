#!/usr/bin/env python3
"""Verify actual object upload and environment isolation using newly owned resources."""
import secrets
import urllib.request
import urllib.parse
from importlib.util import module_from_spec, spec_from_file_location
from pathlib import Path

spec = spec_from_file_location('access', Path(__file__).with_name('test-project-access.py'))
a = module_from_spec(spec)
spec.loader.exec_module(a)


def upload(ticket, content):
    method = ticket['upload_method']
    headers = dict(ticket.get('headers', {}))
    body = content
    if method == 'POST':
        boundary = secrets.token_hex(24)
        parts = []
        for key, value in ticket.get('form_fields', {}).items():
            parts.append(f'--{boundary}\r\nContent-Disposition: form-data; name="{key}"\r\n\r\n{value}\r\n'.encode())
        parts.append(f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="verification.txt"\r\nContent-Type: text/plain\r\n\r\n'.encode() + content + b'\r\n')
        parts.append(f'--{boundary}--\r\n'.encode())
        body = b''.join(parts)
        headers['Content-Type'] = 'multipart/form-data; boundary=' + boundary
    url = urllib.parse.urljoin(a.BASE, ticket['upload_url'])
    req = urllib.request.Request(url, data=body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=30) as response:
            assert response.status in (200, 201, 204)
    except urllib.error.HTTPError as error:
        raise AssertionError(f'Object upload failed: HTTP {error.code}') from None


def main():
    owner, _ = a.register('storageowner')
    member, member_id = a.register('storagemember')
    guest, guest_id = a.register('storageguest')
    project = a.call('POST', '/admin/projects', {'key': 'Storage' + a.SUFFIX, 'name': 'Storage verification'}, owner)['project']
    root = '/admin/projects/' + str(project['id'])
    try:
        for uid, role in [(member_id, 'MEMBER'), (guest_id, 'GUEST')]:
            a.call('POST', root + '/members', {'user_id': uid, 'role': role}, owner)
        envs = [a.call('POST', root + '/environments', {'key': key, 'name': key}, owner)['environment']['id'] for key in ['dev', 'prod']]
        scopes = [f'/admin/environments/{eid}/projects/{project["id"]}/assets' for eid in envs]
        content = b'Kirby object storage verification\n' + secrets.token_bytes(32).hex().encode()
        body = {'filename': 'verification.txt', 'content_type': 'text/plain', 'size': len(content)}
        a.call('POST', scopes[0] + '/presign', body, guest, expected=(403,))
        ticket = a.call('POST', scopes[0] + '/presign', body, member)
        upload(ticket, content)
        complete = {'object_key': ticket['object_key']}
        a.call('POST', scopes[1] + '/complete', complete, member, expected=(400, 403, 404))
        a.call('POST', scopes[0] + '/complete', complete, guest, expected=(403,))
        asset = a.call('POST', scopes[0] + '/complete', complete, member)['asset']
        url = urllib.parse.urljoin(a.BASE, asset['url'])
        with urllib.request.urlopen(url, timeout=30) as response:
            assert response.read() == content, 'Downloaded asset bytes do not match upload'
        print('PASS: real object upload/download; Member allowed; Guest and cross-environment completion denied')
    finally:
        a.call('DELETE', root, token=owner)


if __name__ == '__main__':
    main()
