#!/usr/bin/env python3
"""Exercise project authorization against a fresh Kirby installation; never manages containers."""
import json
import os
import secrets
import re
import urllib.request
import urllib.error
import urllib.parse

BASE = os.environ.get('KIRBY_TEST_URL', 'http://127.0.0.1:18089').rstrip('/')
PASSWORD = 'Test-' + secrets.token_urlsafe(18)
SUFFIX = secrets.token_hex(4)


def call(method, path, body=None, token=None, expected=(200,), api_key=None):
    headers = {'Content-Type': 'application/json', 'Origin': BASE}
    if api_key:
        headers['X-Kirby-API-Key'] = api_key
    if token:
        headers['Authorization'] = 'Bearer ' + token
    request = urllib.request.Request(BASE + '/api' + path, data=None if body is None else json.dumps(body).encode(), headers=headers, method=method)
    try:
        response = urllib.request.urlopen(request, timeout=20)
    except urllib.error.HTTPError as error:
        response = error
    raw = response.read().decode()
    assert response.status in expected, f'{method} {path}: expected {expected}, got {response.status}: {raw}'
    return json.loads(raw) if raw else {}


def account(name):
    return {'username': name + SUFFIX, 'nickname': name, 'email': name + SUFFIX + '@example.test', 'password': PASSWORD}


def register(name):
    reply = call('POST', '/auth/register', account(name))
    return reply['access_token'], reply['user']['id']


def publish_config(owner, pid, eid, key, content):
    scope = f'/admin/environments/{eid}'
    config = call('POST', scope + '/config/create', {'project_id': pid, 'key': key}, owner)['config']
    call('POST', scope + '/config/updateValue', {
        'id': config['id'],
        'value': json.dumps(content),
        'version': config.get('version', 0),
    }, owner)
    snapshot = call('POST', scope + '/snapshot/create', {
        'project_id': pid,
        'config_id': config['id'],
        'description': 'Runtime isolation verification',
    }, owner)['snapshot']
    call('POST', scope + f'/snapshots/{snapshot["id"]}/publish', {'version': snapshot.get('version', 0)}, owner)
    return config


def read_runtime(secret, config_key, expected=(200,), environment=None):
    query = {'key': config_key}
    if environment is not None:
        query['environment_id'] = environment
    return call('GET', '/v1/config?' + urllib.parse.urlencode(query), expected=expected, api_key=secret)


def verify_runtime(owner, project, envs):
    pid = project['id']
    other = call('POST', '/admin/projects', {'key': 'Other' + SUFFIX, 'name': 'Other project'}, owner)['project']
    other_env = call('POST', f'/admin/projects/{other["id"]}/environments', {'key': 'dev', 'name': 'Other dev'}, owner)['environment']['id']
    for eid, value in [(envs[0], 'dev-value'), (envs[1], 'prod-value')]:
        publish_config(owner, pid, eid, 'Shared', value)
    publish_config(owner, pid, envs[1], 'ProdOnly', 'prod-only-secret')
    publish_config(owner, other['id'], other_env, 'Shared', 'other-project-secret')
    path = f'/admin/environments/{envs[0]}/projects/{pid}/api-keys'
    key = call('POST', path, {'name': 'runtime-test'}, owner)
    secret = key['secret']
    assert re.fullmatch(r'[A-Za-z0-9_-]{43}', secret)
    assert 'public_id' not in key['api_key']
    for _ in range(2):
        assert json.loads(read_runtime(secret, 'Shared')['content']) == 'dev-value'
    other_key = call('POST', f'/admin/environments/{other_env}/projects/{other["id"]}/api-keys', {'name': 'other-scope'}, owner)['secret']
    assert json.loads(read_runtime(other_key, 'Shared')['content']) == 'other-project-secret'
    injected = call('GET', '/v1/config?' + urllib.parse.urlencode({'key': 'Shared', 'project': other['key']}), api_key=secret)
    assert json.loads(injected['content']) == 'dev-value'
    read_runtime(secret, 'ProdOnly', expected=(404,))
    # An extra environment parameter cannot redirect the credential to prod.
    assert json.loads(read_runtime(secret, 'Shared', environment=envs[1])['content']) == 'dev-value'
    environments = call('GET', f'/admin/environments?project_id={pid}', token=owner)['list']
    env = next(row for row in environments if str(row['id']) == str(envs[0]))
    env_path = f'/admin/projects/{pid}/environments/{envs[0]}'
    disabled = call('PUT', env_path, {'name': env['name'], 'enabled': False, 'version': env.get('version', 0)}, owner)['environment']
    read_runtime(secret, 'Shared', expected=(401,))
    call('PUT', env_path, {'name': env['name'], 'enabled': True, 'version': disabled.get('version', 0)}, owner)
    assert json.loads(read_runtime(secret, 'Shared')['content']) == 'dev-value'
    rotated = call('POST', path + '/' + str(key['api_key']['id']) + '/rotate', {}, owner)
    assert re.fullmatch(r'[A-Za-z0-9_-]{43}', rotated['secret'])
    assert rotated['secret'] != secret
    read_runtime(secret, 'Shared', expected=(401,))
    secret = rotated['secret']
    assert json.loads(read_runtime(secret, 'Shared')['content']) == 'dev-value'
    call('DELETE', path + '/' + str(key['api_key']['id']), token=owner)
    read_runtime(secret, 'Shared', expected=(401,))
    call('DELETE', f'/admin/projects/{other["id"]}', token=owner)


def main():
    assert not call('GET', '/setup/status')['initialized'], 'Requires a fresh, uninitialized database'
    admin = account('admin')
    call('POST', '/setup/admin', admin)
    admin_token = call('POST', '/auth/login', {'username': admin['email'], 'password': PASSWORD})['access_token']
    assert not call('GET', '/admin/system-settings', token=admin_token)['allow_self_registration']
    call('POST', '/auth/register', account('closed'), expected=(403,))
    call('PUT', '/admin/system-settings', {'allow_self_registration': True}, admin_token)
    owner, owner_id = register('owner')
    maintainer, maintainer_id = register('maintainer')
    member, member_id = register('member')
    guest, guest_id = register('guest')
    project = call('POST', '/admin/projects', {'key': 'Access' + SUFFIX, 'name': 'Access verification'}, owner)['project']
    pid = project['id']
    root = f'/admin/projects/{pid}'
    assert call('GET', root + '/members', token=owner)['list'][0]['role'] == 'OWNER'
    assert not call('GET', '/admin/projects', token=admin_token).get('list')
    call('GET', root, token=admin_token, expected=(403, 404))
    assert call('GET', root + '/users?keyword=member', token=owner)['list']
    for uid, role in [(maintainer_id, 'MAINTAINER'), (member_id, 'MEMBER'), (guest_id, 'GUEST')]:
        call('POST', root + '/members', {'user_id': uid, 'role': role}, owner)
    call('PUT', root + f'/members/{owner_id}', {'role': 'MEMBER'}, maintainer, expected=(403,))
    call('PUT', root + f'/members/{member_id}', {'role': 'OWNER'}, maintainer, expected=(403,))
    call('PUT', root + f'/members/{owner_id}', {'role': 'MEMBER'}, owner, expected=(400, 403, 409))
    call('PUT', root + f'/members/{member_id}', {'role': 'OWNER'}, owner)
    call('PUT', root + f'/members/{member_id}', {'role': 'MEMBER'}, owner)
    envs = [call('POST', root + '/environments', {'key': key, 'name': key}, owner)['environment']['id'] for key in ['dev', 'prod']]
    eid = envs[0]
    scope = f'/admin/environments/{eid}'
    call('POST', root + '/environments', {'key': 'forbidden', 'name': 'forbidden'}, maintainer, expected=(403,))
    matrix = call('GET', scope + '/permissions', token=maintainer)
    for row in matrix['roles']:
        if row['role'] == 'MEMBER':
            row['permissions'] = ['READ']
    call('PUT', scope + '/permissions', matrix, maintainer)
    assert 'config:write' not in call('GET', scope + '/my-permissions', token=member)['permissions']
    assert 'config:write' in call('GET', f'/admin/environments/{envs[1]}/my-permissions', token=member)['permissions']
    config = {'project_id': pid, 'key': 'Blocked'}
    call('POST', scope + '/config/create', config, member, expected=(403,))
    for row in matrix['roles']:
        if row['role'] == 'MEMBER':
            row['permissions'] = ['READ', 'WRITE', 'API_KEY_MANAGE']
    call('PUT', scope + '/permissions', matrix, maintainer)
    assert 'config:write' in call('GET', scope + '/my-permissions', token=member)['permissions']
    call('POST', scope + '/config/create', config, member)
    key = call('POST', scope + f'/projects/{pid}/api-keys', {'name': 'test'}, member)
    call('DELETE', f'/admin/environments/{envs[1]}/projects/{pid}/api-keys/{key["api_key"]["id"]}', token=owner, expected=(403, 404))
    # Management is independently grantable, without configuration READ.
    for row in matrix['roles']:
        if row['role'] == 'MEMBER':
            row['permissions'] = ['API_KEY_MANAGE']
    call('PUT', scope + '/permissions', matrix, maintainer)
    call('GET', scope + f'/projects/{pid}/api-keys', token=member)
    call('POST', scope + '/config/list', {'project_id': pid}, member, expected=(403,))
    for row in matrix['roles']:
        if row['role'] == 'MEMBER':
            row['permissions'] = ['READ']
    call('PUT', scope + '/permissions', matrix, maintainer)
    call('POST', scope + f'/projects/{pid}/api-keys', {'name': 'revoked-permission'}, member, expected=(403,))
    verify_runtime(owner, project, envs)
    invitation = call('POST', root + '/invitations', {'role': 'GUEST'}, maintainer)
    invite_path = '/invitations/' + invitation['token']
    call('GET', invite_path)
    newcomer, newcomer_id = register('invited')
    assert not call('GET', '/admin/projects', token=newcomer).get('list')
    call('POST', invite_path + '/accept', {}, newcomer)
    assert any(str(row['user']['id']) == str(newcomer_id) for row in call('GET', root + '/members', token=owner)['list'])
    call('POST', invite_path + '/accept', {}, newcomer, expected=(400, 403, 404, 409))
    call('DELETE', root, token=maintainer, expected=(403,))
    call('DELETE', root, token=owner)
    assert not call('GET', '/admin/projects', token=owner).get('list')
    print('PASS: published runtime config, cross-project/environment isolation, disabled environment, revoked key,  setup, registration, membership, matrix, environment key scope, invitation acceptance, deletion')


if __name__ == '__main__':
    main()
